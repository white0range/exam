package discovery

import (
	"net"
	"strings"
	"testing"
	"time"

	"exam/internal/cli"
	"exam/internal/format"

	"golang.org/x/net/dns/dnsmessage"
)

func TestRecordStoreBuildsDeepMDNSAssets(t *testing.T) {
	store := newRecordStore()

	mustIngestResource(t, store, resourcePTR(serviceDiscoveryPTR, "_qdiscover._tcp.local.", 10))
	mustIngestResource(t, store, resourcePTR(serviceDiscoveryPTR, "_device-info._tcp.local.", 10))

	mustIngestResource(t, store, resourcePTR("_qdiscover._tcp.local.", "slw-nas._qdiscover._tcp.local.", 10))
	mustIngestResource(t, store, resourcePTR("_device-info._tcp.local.", "slw-nas(AFP)._device-info._tcp.local.", 10))

	mustIngestResource(t, store, resourceSRV("slw-nas._qdiscover._tcp.local.", "slw-nas.local.", 5000, 10))
	mustIngestResource(t, store, resourceTXT("slw-nas._qdiscover._tcp.local.",
		[]string{"accessType=https,accessPort=86,model=TS-X64,displayModel=TS-464C,fwVer=5.2.9,fwBuildNum=20260214"}, 10))
	mustIngestResource(t, store, resourceTXT("slw-nas(AFP)._device-info._tcp.local.", []string{"model=Xserve"}, 10))
	mustIngestResource(t, store, resourceA("slw-nas.local.", "192.168.1.20", 10))

	_, network, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR returned error: %v", err)
	}

	report := filterAndSort(store.assets(), store.ptrAnswers, cli.Options{
		Network:   network,
		PortRange: cli.PortRange{Start: 1, End: 65535},
		Timeout:   time.Second,
	})

	if len(report.Services) != 2 {
		t.Fatalf("unexpected service count: %d", len(report.Services))
	}

	text, err := format.Text(report)
	if err != nil {
		t.Fatalf("Text returned error: %v", err)
	}
	output := string(text)

	expectedFragments := []string{
		"device-info:\nName=slw-nas(AFP)\nIPv4=192.168.1.20\nHostname=slw-nas.local\nTTL=10\nmodel=Xserve\n",
		"5000/tcp qdiscover:\nName=slw-nas\nIPv4=192.168.1.20\nHostname=slw-nas.local\nTTL=10\naccessType=https\naccessPort=86\nmodel=TS-X64\ndisplayModel=TS-464C\nfwVer=5.2.9\nfwBuildNum=20260214\n",
		"_device-info._tcp.local\n",
		"_qdiscover._tcp.local\n",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(output, fragment) {
			t.Fatalf("output missing fragment:\n%s\nfull output:\n%s", fragment, output)
		}
	}
}

func mustIngestResource(t *testing.T, store *recordStore, resource dnsmessage.Resource) {
	t.Helper()
	store.ingestResource(resource)
}

func resourcePTR(name, target string, ttl uint32) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  mustName(name),
			Type:  dnsmessage.TypePTR,
			Class: dnsmessage.ClassINET,
			TTL:   ttl,
		},
		Body: &dnsmessage.PTRResource{PTR: mustName(target)},
	}
}

func resourceSRV(name, target string, port uint16, ttl uint32) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  mustName(name),
			Type:  dnsmessage.TypeSRV,
			Class: dnsmessage.ClassINET,
			TTL:   ttl,
		},
		Body: &dnsmessage.SRVResource{Target: mustName(target), Port: port},
	}
}

func resourceTXT(name string, txt []string, ttl uint32) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  mustName(name),
			Type:  dnsmessage.TypeTXT,
			Class: dnsmessage.ClassINET,
			TTL:   ttl,
		},
		Body: &dnsmessage.TXTResource{TXT: txt},
	}
}

func resourceA(name, ip string, ttl uint32) dnsmessage.Resource {
	parsed := net.ParseIP(ip).To4()
	var raw [4]byte
	copy(raw[:], parsed)
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  mustName(name),
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
			TTL:   ttl,
		},
		Body: &dnsmessage.AResource{A: raw},
	}
}

func mustName(name string) dnsmessage.Name {
	parsed, err := dnsmessage.NewName(name)
	if err != nil {
		panic(err)
	}
	return parsed
}
