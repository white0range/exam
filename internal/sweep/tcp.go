package sweep

import (
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"

	"exam/internal/cli"
)

type Result struct {
	OpenTCP map[string]map[int]struct{}
}

type endpoint struct {
	IP   string
	Port int
}

func TCP(opts cli.Options) (Result, error) {
	return TCPHosts(enumerateHosts(opts.Network), opts.PortRange, opts.Timeout)
}

func TCPHosts(hosts []string, portRange cli.PortRange, timeout time.Duration) (Result, error) {
	result := Result{
		OpenTCP: make(map[string]map[int]struct{}),
	}

	hosts = dedupeHosts(hosts)
	if len(hosts) == 0 {
		return result, nil
	}

	workerCount := runtime.NumCPU() * 64
	if workerCount < 64 {
		workerCount = 64
	}
	if workerCount > 512 {
		workerCount = 512
	}

	dialTimeout := portDialTimeout(timeout)
	jobs := make(chan endpoint, workerCount*2)
	openPorts := make(chan endpoint, workerCount*2)

	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				address := net.JoinHostPort(job.IP, strconv.Itoa(job.Port))
				conn, err := net.DialTimeout("tcp", address, dialTimeout)
				if err != nil {
					continue
				}
				_ = conn.Close()
				openPorts <- job
			}
		}()
	}

	go func() {
		for _, host := range hosts {
			for port := portRange.Start; port <= portRange.End; port++ {
				jobs <- endpoint{IP: host, Port: port}
			}
		}
		close(jobs)
		workers.Wait()
		close(openPorts)
	}()

	for open := range openPorts {
		ports := result.OpenTCP[open.IP]
		if ports == nil {
			ports = make(map[int]struct{})
			result.OpenTCP[open.IP] = ports
		}
		ports[open.Port] = struct{}{}
	}

	return result, nil
}

func (r Result) HasOpenPort(ip string, port int) bool {
	if ip == "" || port <= 0 {
		return false
	}
	ports := r.OpenTCP[ip]
	if ports == nil {
		return false
	}
	_, ok := ports[port]
	return ok
}

func (r Result) HasAnyOpen(ip string) bool {
	if ip == "" {
		return false
	}
	return len(r.OpenTCP[ip]) > 0
}

func enumerateHosts(network *net.IPNet) []string {
	if network == nil {
		return nil
	}

	if ipv4 := network.IP.To4(); ipv4 != nil {
		return enumerateIPv4Hosts(network, ipv4)
	}

	ones, bits := network.Mask.Size()
	if bits == 128 && ones == 128 {
		return []string{network.IP.String()}
	}

	return nil
}

func dedupeHosts(hosts []string) []string {
	seen := make(map[string]struct{}, len(hosts))
	result := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		result = append(result, host)
	}
	return result
}

func enumerateIPv4Hosts(network *net.IPNet, base net.IP) []string {
	mask := network.Mask
	networkIP := base.Mask(mask).To4()
	if networkIP == nil {
		return nil
	}

	start := append(net.IP(nil), networkIP...)
	end := append(net.IP(nil), networkIP...)
	for i := range end {
		end[i] |= ^mask[i]
	}

	ones, bits := mask.Size()
	skipNetwork := bits == 32 && ones <= 30
	skipBroadcast := bits == 32 && ones <= 30

	hosts := make([]string, 0, 32)
	for current := append(net.IP(nil), start...); ; incIPv4(current) {
		if !network.Contains(current) {
			break
		}
		if skipNetwork && current.Equal(start) {
			if current.Equal(end) {
				break
			}
			continue
		}
		if skipBroadcast && current.Equal(end) {
			break
		}
		hosts = append(hosts, current.String())
		if current.Equal(end) {
			break
		}
	}

	return hosts
}

func incIPv4(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
}

func portDialTimeout(total time.Duration) time.Duration {
	switch {
	case total <= 0:
		return 300 * time.Millisecond
	case total < time.Second:
		return total / 2
	case total > 2*time.Second:
		return 500 * time.Millisecond
	default:
		return total / 3
	}
}
