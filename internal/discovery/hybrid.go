package discovery

import (
	"exam/internal/cli"
	"exam/internal/model"
	"exam/internal/sweep"
)

func Scan(opts cli.Options) (model.Report, error) {
	mdnsAssets, ptrAnswers, err := discoverMDNS(opts)
	if err != nil {
		return model.Report{}, err
	}

	scanResult, err := sweep.TCP(opts)
	if err != nil {
		return model.Report{}, err
	}

	return correlateAssets(mdnsAssets, ptrAnswers, scanResult, opts), nil
}

func correlateAssets(assets []model.Asset, ptrAnswers []string, scan sweep.Result, opts cli.Options) model.Report {
	filtered := make([]model.Asset, 0, len(assets))

	for _, original := range assets {
		asset := original
		if asset.Port <= 0 {
			continue
		}
		if !opts.PortRange.Contains(asset.Port) {
			continue
		}
		if !matchesCIDR(asset, opts.Network) {
			continue
		}
		asset.VerifiedOpen = assetHasOpenPort(asset, scan)

		filtered = append(filtered, asset)
	}

	for _, original := range assets {
		asset := original
		if asset.Port > 0 {
			continue
		}
		if !matchesCIDR(asset, opts.Network) {
			continue
		}

		filtered = append(filtered, asset)
	}

	return filterAndSort(filtered, filterPTRAnswers(ptrAnswers, filtered), opts)
}

func assetHasOpenPort(asset model.Asset, scan sweep.Result) bool {
	for _, ip := range assetIPs(asset) {
		if scan.HasOpenPort(ip, asset.Port) {
			return true
		}
	}
	return false
}

func assetIPs(asset model.Asset) []string {
	ips := make([]string, 0, 2)
	if asset.IPv4 != "" {
		ips = append(ips, asset.IPv4)
	}
	if asset.IPv6 != "" {
		ips = append(ips, asset.IPv6)
	}
	return ips
}

func filterPTRAnswers(ptrAnswers []string, assets []model.Asset) []string {
	allowed := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.ServiceType == "" {
			continue
		}
		allowed[normalizeFQDN(asset.ServiceType)] = struct{}{}
	}

	filtered := make([]string, 0, len(ptrAnswers))
	for _, answer := range ptrAnswers {
		if _, ok := allowed[normalizeFQDN(answer)]; ok {
			filtered = append(filtered, answer)
		}
	}
	return filtered
}
