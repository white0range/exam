package format

import (
	"bytes"
	"fmt"

	"exam/internal/model"
)

func Text(report model.Report) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("services:\n")

	for _, asset := range report.Services {
		if asset.Port > 0 {
			fmt.Fprintf(&buf, "%d/%s %s:\n", asset.Port, asset.Protocol, asset.Service)
		} else {
			fmt.Fprintf(&buf, "%s:\n", asset.Service)
		}
		fmt.Fprintf(&buf, "Name=%s\n", asset.Name)
		if asset.IPv4 != "" {
			fmt.Fprintf(&buf, "IPv4=%s\n", asset.IPv4)
		}
		if asset.IPv6 != "" {
			fmt.Fprintf(&buf, "IPv6=%s\n", asset.IPv6)
		}
		if asset.Hostname != "" {
			fmt.Fprintf(&buf, "Hostname=%s\n", displayDNSName(asset.Hostname))
		}
		fmt.Fprintf(&buf, "TTL=%d\n", asset.TTL)

		for _, key := range orderedBannerKeys(asset) {
			fmt.Fprintf(&buf, "%s=%s\n", key, asset.Banner[key])
		}
	}

	buf.WriteString("answers:\n")
	buf.WriteString("PTR:\n")
	for _, answer := range report.PTRAnswers {
		buf.WriteString(displayDNSName(answer))
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

func displayDNSName(name string) string {
	return string(bytes.TrimSuffix([]byte(name), []byte(".")))
}

func orderedBannerKeys(asset model.Asset) []string {
	keys := make([]string, 0, len(asset.Banner))
	seen := make(map[string]struct{}, len(asset.Banner))

	for _, key := range asset.BannerOrder {
		if _, ok := asset.Banner[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	for key := range asset.Banner {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}
