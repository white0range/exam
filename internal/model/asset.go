package model

type Asset struct {
	Name        string            `json:"name"`
	Service     string            `json:"service"`
	ServiceType string            `json:"service_type"`
	Protocol    string            `json:"protocol"`
	Hostname    string            `json:"hostname"`
	Target      string            `json:"target"`
	Port        int               `json:"port"`
	TTL         uint32            `json:"ttl"`
	IPv4        string            `json:"ipv4,omitempty"`
	IPv6        string            `json:"ipv6,omitempty"`
	VerifiedOpen bool             `json:"verified_open,omitempty"`
	Banner      map[string]string `json:"banner,omitempty"`
	BannerOrder []string          `json:"banner_order,omitempty"`
	RawTXT      []string          `json:"raw_txt,omitempty"`
}

type Report struct {
	Services   []Asset  `json:"services"`
	PTRAnswers []string `json:"ptr_answers"`
}
