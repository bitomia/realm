package config

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/viper"

	"github.com/bitomia/realm/common"
)

const regYAML = `
nodes:
  lab1:
    url: http://192.168.1.59:9000
    driver: linux
    registries:
      - host: registry.example.com:5000
        insecure: true
        auth:
          username: admin
          password: secret
      - host: nexus.example.com
        skip_tls_verify: true
        ca_file: /etc/realm/certs/nexus-ca.pem
`

func TestRegistryYAMLParsing(t *testing.T) {
	ResetNodesConfig()
	viper.Reset()
	setDefaults() // must not inject keys AgentConfig no longer has
	viper.SetConfigType("yaml")
	cfg, err := unmarshalConfigHandler(bytes.NewBufferString(regYAML))
	if err != nil {
		t.Fatalf("config parse: %v", err)
	}
	regs := cfg.Nodes["lab1"].Registries
	if len(regs) != 2 {
		t.Fatalf("got %d registries: %+v", len(regs), regs)
	}
	t.Logf("%+v", regs)
	if !regs[0].Insecure {
		t.Error("registries[0].Insecure = false, want true")
	}
	if regs[0].Auth.Username != "admin" || regs[0].Auth.Password != "secret" {
		t.Errorf("auth not parsed: %+v", regs[0].Auth)
	}
	if !regs[1].SkipTLSVerify {
		t.Error("registries[1].SkipTLSVerify = false, want true")
	}
	if regs[1].CAFile == "" {
		t.Error("registries[1].CAFile empty")
	}

	// round-trip over the wire (node config push -> agent)
	b, _ := json.Marshal(regs)
	var back []common.RegistryConfig
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back[0].Insecure {
		t.Errorf("insecure lost in JSON round-trip: %s", b)
	}
}
