package parser

import "testing"

func TestTXTToBanner(t *testing.T) {
	banner := TXTToBanner([]string{
		"path=/",
		"accessType=https",
		"enabled",
	})

	if banner["path"] != "/" {
		t.Fatalf("unexpected path value: %q", banner["path"])
	}
	if banner["accessType"] != "https" {
		t.Fatalf("unexpected accessType value: %q", banner["accessType"])
	}
	if banner["enabled"] != "true" {
		t.Fatalf("unexpected flag value: %q", banner["enabled"])
	}
}

func TestTXTToBannerSplitsCompoundTXT(t *testing.T) {
	banner := TXTToBanner([]string{
		"accessType=https,accessPort=86,model=TS-X64,displayModel=TS-464C,fwVer=5.2.9,fwBuildNum=20260214",
	})

	if banner["accessType"] != "https" {
		t.Fatalf("unexpected accessType value: %q", banner["accessType"])
	}
	if banner["accessPort"] != "86" {
		t.Fatalf("unexpected accessPort value: %q", banner["accessPort"])
	}
	if banner["model"] != "TS-X64" {
		t.Fatalf("unexpected model value: %q", banner["model"])
	}
	if banner["fwBuildNum"] != "20260214" {
		t.Fatalf("unexpected fwBuildNum value: %q", banner["fwBuildNum"])
	}
}

func TestTXTFieldsPreservesFieldOrder(t *testing.T) {
	fields := TXTFields([]string{
		"accessType=https,accessPort=86,model=TS-X64",
		"displayModel=TS-464C",
	})

	if len(fields) != 4 {
		t.Fatalf("unexpected field count: %d", len(fields))
	}
	if fields[0].Key != "accessType" || fields[0].Value != "https" {
		t.Fatalf("unexpected first field: %+v", fields[0])
	}
	if fields[1].Key != "accessPort" || fields[1].Value != "86" {
		t.Fatalf("unexpected second field: %+v", fields[1])
	}
	if fields[2].Key != "model" || fields[2].Value != "TS-X64" {
		t.Fatalf("unexpected third field: %+v", fields[2])
	}
	if fields[3].Key != "displayModel" || fields[3].Value != "TS-464C" {
		t.Fatalf("unexpected fourth field: %+v", fields[3])
	}
}
