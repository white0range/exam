package discovery

import (
	"net"
	"testing"
	"time"

	"exam/internal/cli"
	"exam/internal/model"
)

func TestFilterAndSort(t *testing.T) {
	_, network, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR returned error: %v", err)
	}

	report := filterAndSort([]model.Asset{
		{Port: 5000, IPv4: "192.168.1.20", Service: "http", Name: "b"},
		{Port: 445, IPv4: "192.168.1.10", Service: "smb", Name: "a"},
		{Port: 0, IPv4: "192.168.1.15", Service: "device-info", Name: "meta"},
		{Port: 80, IPv4: "10.0.0.5", Service: "http", Name: "c"},
	}, []string{"_http._tcp.local.", "_smb._tcp.local.", "_http._tcp.local."}, cli.Options{
		Network:   network,
		PortRange: cli.PortRange{Start: 100, End: 6000},
		Timeout:   time.Second,
	})

	if len(report.Services) != 3 {
		t.Fatalf("unexpected service count: %d", len(report.Services))
	}
	if report.Services[0].Service != "device-info" {
		t.Fatalf("expected first asset to keep no-port metadata, got %s", report.Services[0].Service)
	}
	if report.Services[1].Port != 445 {
		t.Fatalf("expected second asset port 445, got %d", report.Services[1].Port)
	}
	if len(report.PTRAnswers) != 2 {
		t.Fatalf("unexpected PTR answer count: %d", len(report.PTRAnswers))
	}
}
