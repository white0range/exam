package discovery

import (
	"net"
	"sort"
	"strings"

	"exam/internal/cli"
	"exam/internal/model"
)

func filterAndSort(assets []model.Asset, ptrAnswers []string, opts cli.Options) model.Report {
	filtered := make([]model.Asset, 0, len(assets))
	for _, asset := range assets {
		if asset.Port > 0 && !opts.PortRange.Contains(asset.Port) {
			continue
		}
		if !matchesCIDR(asset, opts.Network) {
			continue
		}
		filtered = append(filtered, asset)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if (filtered[i].Port == 0) != (filtered[j].Port == 0) {
			return filtered[i].Port == 0
		}
		if filtered[i].Port != filtered[j].Port {
			return filtered[i].Port < filtered[j].Port
		}
		if filtered[i].Service != filtered[j].Service {
			return filtered[i].Service < filtered[j].Service
		}
		return filtered[i].Name < filtered[j].Name
	})

	ptrAnswers = dedupeStrings(ptrAnswers)
	sort.Strings(ptrAnswers)

	return model.Report{
		Services:   filtered,
		PTRAnswers: ptrAnswers,
	}
}

func matchesCIDR(asset model.Asset, network *net.IPNet) bool {
	for _, raw := range []string{asset.IPv4, asset.IPv6} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		ip := net.ParseIP(raw)
		if ip != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
