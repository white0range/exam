package sweep

import (
	"fmt"
	"net"
	"testing"
	"time"

	"exam/internal/cli"
)

func TestTCPDetectsOpenPortOnLocalhost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	_, network, err := net.ParseCIDR("127.0.0.1/32")
	if err != nil {
		t.Fatalf("ParseCIDR returned error: %v", err)
	}

	result, err := TCP(cli.Options{
		Network:   network,
		PortRange: cli.PortRange{Start: port, End: port},
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("TCP returned error: %v", err)
	}

	if !result.HasOpenPort("127.0.0.1", port) {
		t.Fatalf("expected open port %s to be detected", fmt.Sprintf("127.0.0.1:%d", port))
	}
}
