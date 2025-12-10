package dto

type Portmap struct {
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type StartNetworkRequest struct {
	Network string    `json:"network"`
	IPMasq  bool      `json:"ip_masq,omitempty"`
	DNS     bool      `json:"dns,omitempty"`
	PortMap []Portmap `json:"portmap,omitempty"`
}
