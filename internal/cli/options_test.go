package cli

import "testing"

func TestParseOptions(t *testing.T) {
	opts, err := ParseOptions([]string{
		"-cidr", "192.168.1.0/24",
		"-ports", "1-10000",
		"-timeout", "7s",
		"-json",
	})
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if opts.Network.String() != "192.168.1.0/24" {
		t.Fatalf("unexpected network: %s", opts.Network.String())
	}
	if opts.PortRange.Start != 1 || opts.PortRange.End != 10000 {
		t.Fatalf("unexpected port range: %+v", opts.PortRange)
	}
	if !opts.JSON {
		t.Fatal("expected JSON output to be enabled")
	}
}

func TestParsePortRangeRejectsInvalidInput(t *testing.T) {
	if _, err := parsePortRange("200-100"); err == nil {
		t.Fatal("expected parsePortRange to reject reversed range")
	}
}
