package common

// RegistryAuth holds authentication credentials for a container registry.
// Token OR Username/Password should be set, not both.
type RegistryAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

// RegistryConfig holds configuration for a container registry.
type RegistryConfig struct {
	Host     string       `json:"host"` // Registry host (e.g., "ghcr.io", "docker.io", "registry.example.com:5000")
	Auth     RegistryAuth `json:"auth"`
	Insecure bool         `json:"insecure,omitempty"` // Allow HTTP instead of HTTPS

	// Skip verification of the registry's TLS certificate.
	SkipTLSVerify bool `json:"skip_tls_verify,omitempty"`

	// Path to a PEM bundle with additional CAs trusted for this registry.
	CAFile string `json:"ca_file,omitempty"`
}

// FindRegistryConfig returns the registry configuration for a given host, or
// nil when the host is not configured.
func FindRegistryConfig(registries []RegistryConfig, host string) *RegistryConfig {
	for i := range registries {
		if registries[i].Host == host {
			return &registries[i]
		}
	}
	return nil
}
