package common

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// Agents advertise themselves as "<hostname>._realm._tcp.local."
// Resolving ".local" agent hosts in-process avoids depending on the system resolver
const (
	agentMDNSService  = "_realm._tcp"
	agentMDNSDomain   = "local."
	mdnsLookupTimeout = 2 * time.Second
	mdnsCacheTTL      = time.Minute
)

type mdnsCacheEntry struct {
	ips     []net.IP
	expires time.Time
}

var (
	mdnsCache   = map[string]mdnsCacheEntry{}
	mdnsCacheMu sync.Mutex

	mdnsTransportOnce sync.Once
	mdnsTransport     http.RoundTripper
)

func isMDNSHost(host string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSuffix(host, ".")), ".local")
}

func agentMDNSTransport() http.RoundTripper {
	mdnsTransportOnce.Do(func() {
		t := http.DefaultTransport.(*http.Transport).Clone()
		dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil || !isMDNSHost(host) {
				return dialer.DialContext(ctx, network, addr)
			}

			ips, err := lookupAgentMDNS(ctx, host)
			if err != nil {
				// Fall back to the system resolver (e.g. glibc + nss-mdns).
				slog.Debug("mDNS agent lookup failed, using system resolver", "host", host, "error", err)
				return dialer.DialContext(ctx, network, addr)
			}

			var lastErr error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			forgetAgentMDNS(host)
			return nil, lastErr
		}
		mdnsTransport = t
	})
	return mdnsTransport
}

func lookupAgentMDNS(ctx context.Context, host string) ([]net.IP, error) {
	key := strings.ToLower(strings.TrimSuffix(host, "."))

	mdnsCacheMu.Lock()
	if e, ok := mdnsCache[key]; ok && time.Now().Before(e.expires) {
		mdnsCacheMu.Unlock()
		return e.ips, nil
	}
	mdnsCacheMu.Unlock()

	instance := strings.TrimSuffix(key, ".local")

	// A zeroconf Resolver shuts itself down when its lookup context ends, so it is single use.
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("creating mDNS resolver: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, mdnsLookupTimeout)
	defer cancel()

	// Buffered so the resolver loop never blocks on send after we stop reading.
	entries := make(chan *zeroconf.ServiceEntry, 8)
	if err := resolver.Lookup(ctx, instance, agentMDNSService, agentMDNSDomain, entries); err != nil {
		return nil, fmt.Errorf("mDNS lookup of %s: %w", host, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("mDNS lookup of %s: no agent answered: %w", host, ctx.Err())
		case entry, ok := <-entries:
			if !ok {
				return nil, fmt.Errorf("mDNS lookup of %s: no agent answered", host)
			}
			ips := append(append([]net.IP{}, entry.AddrIPv4...), entry.AddrIPv6...)
			if len(ips) == 0 {
				continue
			}
			mdnsCacheMu.Lock()
			mdnsCache[key] = mdnsCacheEntry{ips: ips, expires: time.Now().Add(mdnsCacheTTL)}
			mdnsCacheMu.Unlock()
			return ips, nil
		}
	}
}

func forgetAgentMDNS(host string) {
	mdnsCacheMu.Lock()
	delete(mdnsCache, strings.ToLower(strings.TrimSuffix(host, ".")))
	mdnsCacheMu.Unlock()
}

type DiscoveredAgent struct {
	Instance string   `json:"instance"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	IPs      []net.IP `json:"ips"`
}

func (a DiscoveredAgent) URL() string {
	return fmt.Sprintf("%s://%s", AgentSchemeHTTP, net.JoinHostPort(a.Host, fmt.Sprint(a.Port)))
}

// DiscoverAgents browses the network for agents for the given duration
func DiscoverAgents(ctx context.Context, timeout time.Duration) ([]DiscoveredAgent, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("creating mDNS resolver: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	if err := resolver.Browse(ctx, agentMDNSService, agentMDNSDomain, entries); err != nil {
		return nil, fmt.Errorf("mDNS browse of %s: %w", agentMDNSService, err)
	}

	byInstance := map[string]*DiscoveredAgent{}
	var order []string
	for {
		select {
		case <-ctx.Done():
			agents := make([]DiscoveredAgent, 0, len(order))
			for _, instance := range order {
				agents = append(agents, *byInstance[instance])
			}
			return agents, nil
		case entry, ok := <-entries:
			if !ok {
				// Resolver closes the channel once the context ends.
				entries = nil
				continue
			}
			agent, exists := byInstance[entry.Instance]
			if !exists {
				agent = &DiscoveredAgent{
					Instance: entry.Instance,
					Host:     strings.ToLower(entry.Instance) + ".local",
					Port:     entry.Port,
				}
				byInstance[entry.Instance] = agent
				order = append(order, entry.Instance)
			}
			for _, ip := range append(append([]net.IP{}, entry.AddrIPv4...), entry.AddrIPv6...) {
				if !slices.ContainsFunc(agent.IPs, ip.Equal) {
					agent.IPs = append(agent.IPs, ip)
				}
			}
		}
	}
}
