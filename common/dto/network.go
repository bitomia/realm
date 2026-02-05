package dto

type Portmap struct {
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type NetworkConfig struct {
	Mode    string    `json:"mode,omitempty"`    // Network mode: "bridge" (default) or "host"
	Network string    `json:"network,omitempty"` // Network name (only used in bridge mode)
	IPMasq  bool      `json:"ip_masq,omitempty"`
	DNS     bool      `json:"dns,omitempty"`
	PortMap []Portmap `json:"portmap,omitempty"`
}
