package cli

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	CIDR      string
	Network   *net.IPNet
	PortExpr  string
	PortRange PortRange
	Timeout   time.Duration
	JSON      bool
}

type PortRange struct {
	Start int
	End   int
}

func (r PortRange) Contains(port int) bool {
	return port >= r.Start && port <= r.End
}

func ParseOptions(args []string) (Options, error) {
	fs := flag.NewFlagSet("mdnsmap", flag.ContinueOnError)
	fs.SetOutput(nil)

	var opts Options
	fs.StringVar(&opts.CIDR, "cidr", "", "IPv4/IPv6 CIDR filter, e.g. 192.168.1.0/24")
	fs.StringVar(&opts.PortExpr, "ports", "", "port range filter, e.g. 1-10000")
	fs.DurationVar(&opts.Timeout, "timeout", 5*time.Second, "discovery timeout")
	fs.BoolVar(&opts.JSON, "json", false, "output JSON")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}

	if strings.TrimSpace(opts.CIDR) == "" {
		return Options{}, errors.New("missing required -cidr")
	}
	if strings.TrimSpace(opts.PortExpr) == "" {
		return Options{}, errors.New("missing required -ports")
	}

	_, network, err := net.ParseCIDR(opts.CIDR)
	if err != nil {
		return Options{}, fmt.Errorf("invalid -cidr: %w", err)
	}
	portRange, err := parsePortRange(opts.PortExpr)
	if err != nil {
		return Options{}, fmt.Errorf("invalid -ports: %w", err)
	}

	opts.Network = network
	opts.PortRange = portRange
	return opts, nil
}

func parsePortRange(expr string) (PortRange, error) {
	parts := strings.Split(strings.TrimSpace(expr), "-")
	if len(parts) != 2 {
		return PortRange{}, errors.New("must be in start-end format")
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return PortRange{}, errors.New("invalid start port")
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return PortRange{}, errors.New("invalid end port")
	}
	if start < 1 || end > 65535 || start > end {
		return PortRange{}, errors.New("port range must satisfy 1 <= start <= end <= 65535")
	}

	return PortRange{Start: start, End: end}, nil
}
