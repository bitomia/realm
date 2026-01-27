package cruntime

import (
	"net/http"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/bitomia/realm/common/config"
)

const DefaultDockerHost = "docker.io"

// GetRegistryHost extracts the registry host from an image reference.
// For example:
//
//	"ghcr.io/org/image:tag" -> "ghcr.io"
//	"docker.io/library/nginx" -> "docker.io"
//	"nginx:latest" -> "docker.io"
//	"registry.example.com:5000/image" -> "registry.example.com:5000"
func GetRegistryHost(imageName string) string {
	parts := strings.SplitN(imageName, "/", 2)
	if len(parts) == 1 {
		return DefaultDockerHost
	}

	firstPart := parts[0]
	if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
		return firstPart
	}

	return DefaultDockerHost
}

// findRegistryConfig finds the registry configuration for a given host.
func findRegistryConfig(registries []config.RegistryConfig, host string) *config.RegistryConfig {
	for i := range registries {
		if registries[i].Host == host {
			return &registries[i]
		}
	}
	return nil
}

func GetCredentialsFunc(daemonCfg *config.DaemonConfig) docker.Authorizer {
	return docker.NewDockerAuthorizer(
		docker.WithAuthCreds(func(host string) (string, string, error) {
			if len(daemonCfg.Registries) == 0 {
				return "", "", nil
			}

			regCfg := findRegistryConfig(daemonCfg.Registries, host)
			if regCfg == nil {
				return "", "", nil
			}

			// Token-based auth (e.g., GitHub PAT)
			// Use token as password with a placeholder username
			if regCfg.Auth.Token != "" {
				return "x-access-token", regCfg.Auth.Token, nil
			}

			// Username/password auth
			if regCfg.Auth.Username != "" {
				return regCfg.Auth.Username, regCfg.Auth.Password, nil
			}

			return "", "", nil
		}),
	)
}

func createRegistryHosts(daemonCfg *config.DaemonConfig) docker.RegistryHosts {
	authorizer := GetCredentialsFunc(daemonCfg)

	// default hosts with anonymous authorizer for public registries like Docker Hub
	defaultHosts := docker.ConfigureDefaultRegistries(
		docker.WithAuthorizer(docker.NewDockerAuthorizer()),
	)

	return func(host string) ([]docker.RegistryHost, error) {
		regCfg := findRegistryConfig(daemonCfg.Registries, host)

		// if no config for this host, use default configuration with anonymous auth
		if regCfg == nil {
			return defaultHosts(host)
		}

		scheme := "https"
		if regCfg.Insecure {
			scheme = "http"
		}

		registryHost := docker.RegistryHost{
			Host:         host,
			Scheme:       scheme,
			Path:         "/v2",
			Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
			Authorizer:   authorizer,
			Client:       http.DefaultClient,
		}

		return []docker.RegistryHost{registryHost}, nil
	}
}

func GetPullOptions(daemonCfg *config.DaemonConfig) []containerd.RemoteOpt {
	opts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
	}

	if len(daemonCfg.Registries) == 0 {
		return opts
	}

	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: createRegistryHosts(daemonCfg),
	})

	opts = append(opts, containerd.WithResolver(resolver))

	return opts
}
