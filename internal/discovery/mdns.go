package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"exam/internal/cli"
	"exam/internal/model"
	"exam/internal/parser"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	mdnsIPv4Addr        = "224.0.0.251"
	mdnsIPv6Addr        = "ff02::fb"
	mdnsPort            = 5353
	serviceDiscoveryPTR = "_services._dns-sd._udp.local."
)

type mdnsClient struct {
	listeners []*mdnsListener
}

type mdnsListener struct {
	rawConn     net.PacketConn
	sendPacket  func([]byte) bool
	description string
}

type mdnsQuery struct {
	Name string
	Type dnsmessage.Type
}

type recordStore struct {
	serviceTypes map[string]struct{}
	instances    map[string]*instanceRecord
	hostIPs      map[string]*hostIPs
	ptrAnswers   []string
}

type instanceRecord struct {
	InstanceFQDN string
	ServiceType  string
	Name         string
	Hostname     string
	Port         int
	Protocol     string
	Service      string
	TTL          uint32
	RawTXT       []string
	Banner       map[string]string
	BannerOrder  []string
}

type hostIPs struct {
	IPv4 []string
	IPv6 []string
}

func discoverMDNS(opts cli.Options) ([]model.Asset, []string, error) {
	client, err := newMDNSClient()
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	store := newRecordStore()

	if err := client.queryBatchAndCollect([]mdnsQuery{
		{Name: serviceDiscoveryPTR, Type: dnsmessage.TypePTR},
	}, opts.Timeout, map[string]struct{}{normalizeFQDN(serviceDiscoveryPTR): {}}, store); err != nil {
		return nil, nil, err
	}

	serviceTypes := sortedKeys(store.serviceTypes)
	serviceQueries := make([]mdnsQuery, 0, len(serviceTypes))
	serviceMatcher := make(map[string]struct{}, len(serviceTypes))
	for _, serviceType := range serviceTypes {
		serviceQueries = append(serviceQueries, mdnsQuery{Name: serviceType, Type: dnsmessage.TypePTR})
		serviceMatcher[normalizeFQDN(serviceType)] = struct{}{}
	}
	if err := client.queryBatchAndCollect(serviceQueries, opts.Timeout, serviceMatcher, store); err != nil {
		return nil, nil, err
	}

	instanceNames := store.instanceFQDNs()
	instanceQueries := make([]mdnsQuery, 0, len(instanceNames)*2)
	instanceMatcher := make(map[string]struct{}, len(instanceNames))
	for _, instance := range instanceNames {
		instanceQueries = append(instanceQueries,
			mdnsQuery{Name: instance, Type: dnsmessage.TypeSRV},
			mdnsQuery{Name: instance, Type: dnsmessage.TypeTXT},
		)
		instanceMatcher[normalizeFQDN(instance)] = struct{}{}
	}
	if err := client.queryBatchAndCollect(instanceQueries, opts.Timeout, instanceMatcher, store); err != nil {
		return nil, nil, err
	}

	hostnames := store.hostnames()
	hostQueries := make([]mdnsQuery, 0, len(hostnames)*2)
	hostMatcher := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		hostQueries = append(hostQueries,
			mdnsQuery{Name: hostname, Type: dnsmessage.TypeA},
			mdnsQuery{Name: hostname, Type: dnsmessage.TypeAAAA},
		)
		hostMatcher[normalizeFQDN(hostname)] = struct{}{}
	}
	if err := client.queryBatchAndCollect(hostQueries, opts.Timeout/2, hostMatcher, store); err != nil {
		return nil, nil, err
	}

	return store.assets(), dedupeStrings(store.ptrAnswers), nil
}

func newMDNSClient() (*mdnsClient, error) {
	ifaces, err := multicastInterfaces()
	if err != nil {
		return nil, err
	}

	listeners := make([]*mdnsListener, 0, 2)
	var errs []error

	if listener, err := newIPv4Listener(ifaces); err == nil {
		listeners = append(listeners, listener)
	} else {
		errs = append(errs, err)
	}
	if listener, err := newIPv6Listener(ifaces); err == nil {
		listeners = append(listeners, listener)
	} else {
		errs = append(errs, err)
	}

	if len(listeners) == 0 {
		return nil, errors.Join(errs...)
	}

	return &mdnsClient{listeners: listeners}, nil
}

func newIPv4Listener(ifaces []*net.Interface) (*mdnsListener, error) {
	rawConn, err := listenPacket("udp4", fmt.Sprintf("0.0.0.0:%d", mdnsPort))
	if err != nil {
		return nil, fmt.Errorf("listen IPv4 mDNS socket failed: %w", err)
	}

	packetConn := ipv4.NewPacketConn(rawConn)
	group := &net.UDPAddr{IP: net.ParseIP(mdnsIPv4Addr), Port: mdnsPort}
	joined := 0
	for _, ifi := range ifaces {
		if err := packetConn.JoinGroup(ifi, group); err == nil {
			joined++
		}
	}
	if joined == 0 {
		rawConn.Close()
		return nil, errors.New("no multicast-capable interface could join the IPv4 mDNS group")
	}

	return &mdnsListener{
		rawConn:     rawConn,
		description: "IPv4",
		sendPacket: func(packet []byte) bool {
			sent := false
			for _, ifi := range ifaces {
				if err := packetConn.SetMulticastInterface(ifi); err != nil {
					continue
				}
				if _, err := packetConn.WriteTo(packet, nil, group); err == nil {
					sent = true
				}
			}
			return sent
		},
	}, nil
}

func newIPv6Listener(ifaces []*net.Interface) (*mdnsListener, error) {
	rawConn, err := listenPacket("udp6", fmt.Sprintf("[::]:%d", mdnsPort))
	if err != nil {
		return nil, fmt.Errorf("listen IPv6 mDNS socket failed: %w", err)
	}

	packetConn := ipv6.NewPacketConn(rawConn)
	group := &net.UDPAddr{IP: net.ParseIP(mdnsIPv6Addr), Port: mdnsPort}
	joined := 0
	for _, ifi := range ifaces {
		if err := packetConn.JoinGroup(ifi, group); err == nil {
			joined++
		}
	}
	if joined == 0 {
		rawConn.Close()
		return nil, errors.New("no multicast-capable interface could join the IPv6 mDNS group")
	}

	return &mdnsListener{
		rawConn:     rawConn,
		description: "IPv6",
		sendPacket: func(packet []byte) bool {
			sent := false
			for _, ifi := range ifaces {
				if err := packetConn.SetMulticastInterface(ifi); err != nil {
					continue
				}
				if _, err := packetConn.WriteTo(packet, nil, group); err == nil {
					sent = true
				}
			}
			return sent
		},
	}, nil
}

func listenPacket(network, address string) (net.PacketConn, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, rawConn syscall.RawConn) error {
			var controlErr error
			if err := rawConn.Control(func(fd uintptr) {
				controlErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			}); err != nil {
				return err
			}
			return controlErr
		},
	}
	return lc.ListenPacket(context.Background(), network, address)
}

func (c *mdnsClient) Close() error {
	var errs []error
	for _, listener := range c.listeners {
		if err := listener.rawConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *mdnsClient) queryBatchAndCollect(queries []mdnsQuery, timeout time.Duration, matcher map[string]struct{}, store *recordStore) error {
	if len(queries) == 0 {
		return nil
	}
	if timeout <= 0 {
		timeout = time.Second
	}

	sent := false
	for _, query := range dedupeQueries(queries) {
		packet, err := buildQueryPacket(query.Name, query.Type)
		if err != nil {
			return err
		}
		for _, listener := range c.listeners {
			if listener.sendPacket(packet) {
				sent = true
			}
		}
	}
	if !sent {
		return errors.New("unable to send mDNS queries on any listener")
	}

	messages, err := c.collectResponses(time.Now().Add(timeout))
	if err != nil {
		return err
	}
	for _, msg := range messages {
		if messageMatchesQueries(msg, matcher) {
			store.ingestMessage(msg)
		}
	}

	return nil
}

func (c *mdnsClient) collectResponses(deadline time.Time) ([]dnsmessage.Message, error) {
	type result struct {
		msg dnsmessage.Message
		err error
	}

	results := make(chan result, len(c.listeners)*8)
	var wg sync.WaitGroup

	for _, listener := range c.listeners {
		wg.Add(1)
		go func(listener *mdnsListener) {
			defer wg.Done()

			buffer := make([]byte, 65535)
			for {
				if err := listener.rawConn.SetReadDeadline(deadline); err != nil {
					results <- result{err: fmt.Errorf("%s deadline failed: %w", listener.description, err)}
					return
				}

				n, _, err := listener.rawConn.ReadFrom(buffer)
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						return
					}
					results <- result{err: fmt.Errorf("%s read failed: %w", listener.description, err)}
					return
				}

				var msg dnsmessage.Message
				if err := msg.Unpack(buffer[:n]); err != nil {
					continue
				}
				if !msg.Header.Response {
					continue
				}
				results <- result{msg: msg}
			}
		}(listener)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	messages := make([]dnsmessage.Message, 0, 16)
	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, result.err)
			continue
		}
		messages = append(messages, result.msg)
	}

	if len(messages) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return messages, nil
}

func buildQueryPacket(name string, qtype dnsmessage.Type) ([]byte, error) {
	fqdn := ensureDot(name)
	queryName, err := dnsmessage.NewName(fqdn)
	if err != nil {
		return nil, fmt.Errorf("invalid DNS name %q: %w", name, err)
	}

	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			Response:           false,
			RecursionDesired:   false,
			Authoritative:      false,
			Truncated:          false,
			RecursionAvailable: false,
		},
		Questions: []dnsmessage.Question{
			{
				Name:  queryName,
				Type:  qtype,
				Class: dnsmessage.ClassINET,
			},
		},
	}
	return msg.Pack()
}

func multicastInterfaces() ([]*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces failed: %w", err)
	}

	selected := make([]*net.Interface, 0, len(ifaces))
	for i := range ifaces {
		ifi := ifaces[i]
		if ifi.Flags&net.FlagUp == 0 {
			continue
		}
		if ifi.Flags&net.FlagMulticast == 0 {
			continue
		}
		if ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		selected = append(selected, &ifi)
	}

	if len(selected) == 0 {
		for i := range ifaces {
			if ifaces[i].Flags&net.FlagMulticast != 0 {
				selected = append(selected, &ifaces[i])
			}
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("no multicast interface found")
	}

	return selected, nil
}

func newRecordStore() *recordStore {
	return &recordStore{
		serviceTypes: make(map[string]struct{}),
		instances:    make(map[string]*instanceRecord),
		hostIPs:      make(map[string]*hostIPs),
	}
}

func (s *recordStore) ingestMessage(msg dnsmessage.Message) {
	for _, resource := range appendAllResources(msg) {
		s.ingestResource(resource)
	}
}

func appendAllResources(msg dnsmessage.Message) []dnsmessage.Resource {
	resources := make([]dnsmessage.Resource, 0, len(msg.Answers)+len(msg.Authorities)+len(msg.Additionals))
	resources = append(resources, msg.Answers...)
	resources = append(resources, msg.Authorities...)
	resources = append(resources, msg.Additionals...)
	return resources
}

func (s *recordStore) ingestResource(resource dnsmessage.Resource) {
	name := normalizeFQDN(resource.Header.Name.String())
	switch body := resource.Body.(type) {
	case *dnsmessage.PTRResource:
		ptr := normalizeFQDN(body.PTR.String())
		if name == normalizeFQDN(serviceDiscoveryPTR) {
			s.serviceTypes[ptr] = struct{}{}
			s.ptrAnswers = append(s.ptrAnswers, ptr)
			return
		}

		instance := s.instance(ptr)
		instance.ServiceType = name
		instance.Name = instanceDisplayName(ptr, name)
		instance.Service, instance.Protocol = decodeServiceType(name)
		s.ptrAnswers = append(s.ptrAnswers, name)

	case *dnsmessage.SRVResource:
		instance := s.instance(name)
		instance.InstanceFQDN = name
		if instance.ServiceType == "" {
			instance.ServiceType = guessServiceType(name)
			instance.Service, instance.Protocol = decodeServiceType(instance.ServiceType)
		}
		if instance.Name == "" {
			instance.Name = instanceDisplayName(name, instance.ServiceType)
		}
		instance.Hostname = normalizeFQDN(body.Target.String())
		instance.Port = int(body.Port)
		instance.TTL = maxTTL(instance.TTL, resource.Header.TTL)

	case *dnsmessage.TXTResource:
		instance := s.instance(name)
		if instance.ServiceType == "" {
			instance.ServiceType = guessServiceType(name)
			instance.Service, instance.Protocol = decodeServiceType(instance.ServiceType)
		}
		if instance.Name == "" {
			instance.Name = instanceDisplayName(name, instance.ServiceType)
		}
		instance.RawTXT = mergeUnique(instance.RawTXT, body.TXT...)
		if len(instance.Banner) == 0 {
			instance.Banner = make(map[string]string)
		}
		for _, field := range parser.TXTFields(body.TXT) {
			if _, ok := instance.Banner[field.Key]; !ok {
				instance.BannerOrder = append(instance.BannerOrder, field.Key)
			}
			instance.Banner[field.Key] = field.Value
		}
		instance.TTL = maxTTL(instance.TTL, resource.Header.TTL)

	case *dnsmessage.AResource:
		host := s.host(name)
		host.IPv4 = mergeUnique(host.IPv4, net.IP(body.A[:]).String())

	case *dnsmessage.AAAAResource:
		host := s.host(name)
		host.IPv6 = mergeUnique(host.IPv6, net.IP(body.AAAA[:]).String())
	}
}

func (s *recordStore) instance(name string) *instanceRecord {
	name = normalizeFQDN(name)
	record, ok := s.instances[name]
	if ok {
		return record
	}
	record = &instanceRecord{InstanceFQDN: name}
	s.instances[name] = record
	return record
}

func (s *recordStore) host(name string) *hostIPs {
	name = normalizeFQDN(name)
	record, ok := s.hostIPs[name]
	if ok {
		return record
	}
	record = &hostIPs{}
	s.hostIPs[name] = record
	return record
}

func (s *recordStore) instanceFQDNs() []string {
	names := make([]string, 0, len(s.instances))
	for name := range s.instances {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (s *recordStore) hostnames() []string {
	names := make([]string, 0, len(s.instances))
	for _, instance := range s.instances {
		for _, hostname := range instanceCandidateHostnames(instance) {
			names = append(names, hostname)
		}
	}
	names = dedupeStrings(names)
	slices.Sort(names)
	return names
}

func (s *recordStore) assets() []model.Asset {
	assets := make([]model.Asset, 0, len(s.instances))
	for _, instance := range s.instances {
		hostname, host := s.resolveHost(instance)
		asset := model.Asset{
			Name:        instance.Name,
			Service:     instance.Service,
			ServiceType: instance.ServiceType,
			Protocol:    defaultString(instance.Protocol, "tcp"),
			Hostname:    hostname,
			Target:      hostname,
			Port:        instance.Port,
			TTL:         instance.TTL,
			Banner:      instance.Banner,
			BannerOrder: append([]string(nil), instance.BannerOrder...),
			RawTXT:      instance.RawTXT,
		}
		if host != nil {
			if len(host.IPv4) > 0 {
				asset.IPv4 = host.IPv4[0]
			}
			if len(host.IPv6) > 0 {
				asset.IPv6 = host.IPv6[0]
			}
		}
		if asset.Name == "" {
			asset.Name = strings.TrimSuffix(instance.InstanceFQDN, "."+instance.ServiceType)
			asset.Name = strings.TrimSuffix(asset.Name, ".")
		}
		if asset.Service == "" {
			asset.Service, asset.Protocol = decodeServiceType(asset.ServiceType)
		}
		assets = append(assets, asset)
	}
	return assets
}

func (s *recordStore) resolveHost(instance *instanceRecord) (string, *hostIPs) {
	for _, hostname := range instanceCandidateHostnames(instance) {
		host := s.hostIPs[normalizeFQDN(hostname)]
		if host == nil {
			continue
		}
		if len(host.IPv4) > 0 || len(host.IPv6) > 0 {
			return hostname, host
		}
	}
	candidates := instanceCandidateHostnames(instance)
	if len(candidates) > 0 {
		return candidates[0], s.hostIPs[normalizeFQDN(candidates[0])]
	}
	return instance.Hostname, s.hostIPs[normalizeFQDN(instance.Hostname)]
}

func instanceCandidateHostnames(instance *instanceRecord) []string {
	candidates := make([]string, 0, 3)
	if instance.Hostname != "" {
		candidates = append(candidates, normalizeFQDN(instance.Hostname))
	}

	nameSources := []string{
		instance.Name,
		instanceDisplayName(instance.InstanceFQDN, instance.ServiceType),
	}
	for _, source := range nameSources {
		hostname := guessHostnameFromName(source)
		if hostname == "" {
			continue
		}
		candidates = append(candidates, hostname)
	}

	return dedupeStrings(candidates)
}

func instanceDisplayName(instanceFQDN, serviceType string) string {
	instanceFQDN = normalizeFQDN(instanceFQDN)
	serviceType = normalizeFQDN(serviceType)
	if serviceType != "" {
		suffix := "." + serviceType
		if strings.HasSuffix(instanceFQDN, suffix) {
			return strings.TrimSuffix(instanceFQDN, suffix)
		}
	}
	return strings.TrimSuffix(instanceFQDN, ".local")
}

func guessServiceType(instanceFQDN string) string {
	parts := strings.Split(normalizeFQDN(instanceFQDN), ".")
	for i := 0; i < len(parts)-1; i++ {
		if strings.HasPrefix(parts[i], "_") && i+2 < len(parts) && strings.HasPrefix(parts[i+1], "_") {
			return strings.Join(parts[i:], ".")
		}
	}
	return ""
}

func guessHostnameFromName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.Index(name, "("); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	name = strings.Trim(name, ".")
	if name == "" {
		return ""
	}
	if strings.Contains(name, ".") {
		return ensureDot(name)
	}
	return ensureDot(name + ".local")
}

func decodeServiceType(serviceType string) (service string, protocol string) {
	serviceType = normalizeFQDN(serviceType)
	parts := strings.Split(serviceType, ".")
	if len(parts) >= 2 {
		service = strings.TrimPrefix(parts[0], "_")
		protocol = strings.TrimPrefix(parts[1], "_")
	}
	return strings.TrimSuffix(service, "."), strings.TrimSuffix(protocol, ".")
}

func normalizeFQDN(name string) string {
	return ensureDot(strings.TrimSpace(name))
}

func ensureDot(name string) string {
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

func mergeUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		if addition == "" {
			continue
		}
		if slices.Contains(values, addition) {
			continue
		}
		values = append(values, addition)
	}
	return values
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}

func dedupeQueries(queries []mdnsQuery) []mdnsQuery {
	seen := make(map[string]struct{}, len(queries))
	result := make([]mdnsQuery, 0, len(queries))
	for _, query := range queries {
		key := normalizeFQDN(query.Name) + "|" + query.Type.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, query)
	}
	return result
}

func messageMatchesQueries(msg dnsmessage.Message, names map[string]struct{}) bool {
	if len(names) == 0 {
		return true
	}
	for _, resource := range appendAllResources(msg) {
		if _, ok := names[normalizeFQDN(resource.Header.Name.String())]; ok {
			return true
		}
	}
	return false
}

func maxTTL(left, right uint32) uint32 {
	if right > left {
		return right
	}
	return left
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
