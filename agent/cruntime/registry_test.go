package cruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/containerd/containerd/remotes/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/bitomia/realm/common"
)

// minimal OCI registry serving a single manifest over plain HTTP
func fakeRegistry(t *testing.T, wantAuth string) *httptest.Server {
	t.Helper()
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000", Size: 0},
	}
	manifest.SchemaVersion = 2
	body, _ := json.Marshal(manifest)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("registry request: %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		if wantAuth != "" && r.Header.Get("Authorization") != wantAuth {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/manifests/latest"):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", "sha256:1111111111111111111111111111111111111111111111111111111111111111")
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			if r.Method == http.MethodGet {
				_, _ = w.Write(body)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func resolve(t *testing.T, regs []common.RegistryConfig, ref string) (string, error) {
	t.Helper()
	r := docker.NewResolver(docker.ResolverOptions{Hosts: createRegistryHosts(regs)})
	name, _, err := r.Resolve(context.Background(), ref)
	return name, err
}

func TestInsecureTrueUsesHTTP(t *testing.T) {
	srv := fakeRegistry(t, "")
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	if _, err := resolve(t, []common.RegistryConfig{{Host: host, Insecure: true}}, host+"/foo:latest"); err != nil {
		t.Fatalf("insecure:true resolve failed: %v", err)
	}
}

func TestInsecureFalseUsesHTTPS(t *testing.T) {
	srv := fakeRegistry(t, "")
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	_, err := resolve(t, []common.RegistryConfig{{Host: host, Insecure: false}}, host+"/foo:latest")
	if err == nil {
		t.Fatal("expected https attempt against http server to fail, got success")
	}
	t.Logf("insecure:false error (expected): %v", err)
}

func TestInsecureWithBasicAuth(t *testing.T) {
	// admin:secret
	srv := fakeRegistry(t, "Basic YWRtaW46c2VjcmV0")
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	regs := []common.RegistryConfig{{
		Host:     host,
		Insecure: true,
		Auth:     common.RegistryAuth{Username: "admin", Password: "secret"},
	}}
	if _, err := resolve(t, regs, host+"/foo:latest"); err != nil {
		t.Fatalf("insecure+basic auth resolve failed: %v", err)
	}
}

func TestInsecureWithTokenAuth(t *testing.T) {
	// x-access-token:ghp_test
	srv := fakeRegistry(t, "Basic eC1hY2Nlc3MtdG9rZW46Z2hwX3Rlc3Q=")
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	regs := []common.RegistryConfig{{
		Host:     host,
		Insecure: true,
		Auth:     common.RegistryAuth{Token: "ghp_test"},
	}}
	if _, err := resolve(t, regs, host+"/foo:latest"); err != nil {
		t.Fatalf("insecure+token auth resolve failed: %v", err)
	}
}
