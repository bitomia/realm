package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/bitomia/realm/config"
)

const BasicCaddyConfig string = `{ "listen": [ ":80", ":443" ], "routes": [], "tls_connection_policies": [] }`

type ProxyOpts struct {
	Hosts         []string `json:"hosts"`
	Upstream      string   `json:"upstream"`
	HttpUpstream  bool     `json:"http_upstream"`
	HttpsUpstream bool     `json:"https_upstream"`
}

func getRouteConfig(containerName string, hosts []string, upstream string) string {
	hostsStr, _ := json.Marshal(hosts)
	return fmt.Sprintf(`
  {
    "@id": "%s",
    "match": [ { "host": %s } ],
    "handle": [{
		  "handler": "reverse_proxy",
		  "upstreams": [{ "dial": "%s" }],
      "transport": {
          "protocol": "http",
          "tls": {
              "insecure_skip_verify": true
          }
      }
	  }]
  }
  `, containerName, hostsStr, upstream)
}

func getHTTPRouteConfig(containerName string, hosts []string, upstream string, ssl bool) string {
	hostsStr, _ := json.Marshal(hosts)
	sslConfig := ""
	if ssl {
		sslConfig = `"transport": { "protocol": "http", "tls": { "insecure_skip_verify": true } },`
	}
	return fmt.Sprintf(`
  {
    "@id": "%s",
    "match": [ { "host": %s } ],
    "handle": [{
		  "handler": "reverse_proxy",
      "headers": {
        "request": {
          "set": {
            "X-Forwarded-Proto": [
              "https"
            ]
          }
        }
      },
      %s
		  "upstreams": [{ "dial": "%s" }]
	  }]
  }
  `, containerName, hostsStr, sslConfig, upstream)
}

func createMasterRoute(containerName string, hosts []string, upstream string, isHTTP bool) error {
	masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
	if len(masterCaddyURL) == 0 {
		return errors.New("env var REALM_MASTER_CADDY_URL not found")
	}
	masterCaddyURL = fmt.Sprintf("http://%s/config/apps/http/servers/master/routes", masterCaddyURL)

	var caddyRequest string
	if isHTTP {
		caddyRequest = getHTTPRouteConfig(containerName, hosts, upstream, false)
	} else {
		caddyRequest = getRouteConfig(containerName, hosts, upstream)
	}

	req, err := http.NewRequest("POST", masterCaddyURL, strings.NewReader(caddyRequest))
	if err != nil {
		slog.Error("Caddy request failed", "request", caddyRequest)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("error on createMasterRoute %s. Master caddy returned %s", containerName, resp.Status)
	}
	return nil
}

func deleteMasterRoute(containerName string) error {
	masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
	if len(masterCaddyURL) == 0 {
		return errors.New("env var REALM_MASTER_CADDY_URL not found")
	}
	masterCaddyURL = fmt.Sprintf("http://%s/id/%s", masterCaddyURL, containerName)

	req, err := http.NewRequest("DELETE", masterCaddyURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func Initialize() {
	slog.Info("Initializing Caddy proxy connection")

	result, err := HasCaddyConfig()
	if err != nil {
		slog.Error("Error connecting to Caddy", "caddy", err)
		os.Exit(1)
	}

	if result {
		slog.Info("Caddy config already initialized")
		return
	} else {
		slog.Info("Caddy config not found. Creating new config.")

		DeleteCaddyDefaultServer()

		caddyURL := config.Get().Daemon.LocalCaddyUrl
		caddyURL = fmt.Sprintf("http://%s/config/apps/http/servers/realm", caddyURL)

		basicCaddyConfigReader := strings.NewReader(BasicCaddyConfig)
		req, err := http.NewRequest("POST", caddyURL, basicCaddyConfigReader)
		if err != nil {
			slog.Error("Error connecting to Caddy", "error", err)
			os.Exit(1)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("Error sending initialization to Caddy", "error", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
	}

}

func DeleteCaddyDefaultServer() {
	caddyURL := config.Get().Daemon.LocalCaddyUrl
	caddyURL = fmt.Sprintf("http://%s/config/apps/http/servers/srv0", caddyURL)

	req, err := http.NewRequest("DELETE", caddyURL, nil)
	if err != nil {
		slog.Error("Error connecting to Caddy", "error", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error deleting all caddy servers", "error", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
}

func HasCaddyConfig() (bool, error) {
	caddyBaseURL := config.Get().Daemon.LocalCaddyUrl
	caddyURL := fmt.Sprintf("http://%s/config/apps/http/servers/realm", caddyBaseURL)

	req, err := http.NewRequest("GET", caddyURL, nil)
	if err != nil {
		slog.Error("Error connecting to Caddy", "error", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error getting Caddy config", "error", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var body []byte = make([]byte, 10)
	io.LimitReader(resp.Body, 10).Read(body)

	if len(body) == 0 || strings.Contains(string(body), "null") {
		return false, nil
	} else {
		return true, nil
	}
}
