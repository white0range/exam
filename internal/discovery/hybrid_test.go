package discovery

import (
	"net"
	"testing"
	"time"

	"exam/internal/cli"
	"exam/internal/model"
	"exam/internal/sweep"
)

func TestCorrelateAssetsKeepsPortBoundServicesAndMarksVerification(t *testing.T) {
	_, network, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR returned error: %v", err)
	}

	report := correlateAssets([]model.Asset{
		{
			Name:        "slw-nas",
			Service:     "http",
			ServiceType: "_http._tcp.local.",
			Protocol:    "tcp",
			Hostname:    "slw-nas.local.",
			Port:        5000,
			IPv4:        "192.168.1.20",
		},
		{
			Name:        "slw-nas",
			Service:     "smb",
			ServiceType: "_smb._tcp.local.",
			Protocol:    "tcp",
			Hostname:    "slw-nas.local.",
			Port:        445,
			IPv4:        "192.168.1.20",
		},
	}, []string{"_http._tcp.local.", "_smb._tcp.local."}, sweep.Result{
		OpenTCP: map[string]map[int]struct{}{
			"192.168.1.20": {5000: {}},
		},
	}, cli.Options{
		Network:   network,
		PortRange: cli.PortRange{Start: 1, End: 6000},
		Timeout:   time.Second,
	})

	if len(report.Services) != 2 {
		t.Fatalf("unexpected service count: %d", len(report.Services))
	}
	if report.Services[0].Service != "smb" {
		t.Fatalf("expected smb asset first after port sort, got %s", report.Services[0].Service)
	}
	if report.Services[0].VerifiedOpen {
		t.Fatal("expected smb asset to remain but not be marked verified")
	}
	if !report.Services[1].VerifiedOpen {
		t.Fatal("expected http asset to be marked verified")
	}
	if len(report.PTRAnswers) != 2 {
		t.Fatalf("unexpected PTR answers: %+v", report.PTRAnswers)
	}
}

func TestCorrelateAssetsKeepsMetadataWithinTargetCIDR(t *testing.T) {
	_, network, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR returned error: %v", err)
	}

	report := correlateAssets([]model.Asset{
		{
			Name:        "slw-nas(AFP)",
			Service:     "device-info",
			ServiceType: "_device-info._tcp.local.",
			Hostname:    "slw-nas.local.",
			IPv4:        "192.168.1.20",
		},
		{
			Name:        "slw-nas(AFP)",
			Service:     "afpovertcp",
			ServiceType: "_afpovertcp._tcp.local.",
			Protocol:    "tcp",
			Hostname:    "slw-nas.local.",
			Port:        548,
			IPv4:        "192.168.1.20",
		},
	}, []string{"_device-info._tcp.local.", "_afpovertcp._tcp.local."}, sweep.Result{
		OpenTCP: map[string]map[int]struct{}{},
	}, cli.Options{
		Network:   network,
		PortRange: cli.PortRange{Start: 1, End: 1000},
		Timeout:   time.Second,
	})

	if len(report.Services) != 2 {
		t.Fatalf("unexpected service count: %d", len(report.Services))
	}
	if report.Services[0].Service != "device-info" {
		t.Fatalf("expected metadata service first, got %s", report.Services[0].Service)
	}
	if report.Services[1].VerifiedOpen {
		t.Fatal("expected afpovertcp asset to remain but not be marked verified")
	}
}
