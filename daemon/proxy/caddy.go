package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/bitomia/realm/config"
)

type CaddyMatch struct {
	Host []string `json:"host"`
}

type CaddyUpstream struct {
	Dial string `json:"dial"`
}

type CaddyHandle struct {
	Handler   string          `json:"handler"`
	Upstreams []CaddyUpstream `json:"upstreams"`
}

type CaddyRoute struct {
	Match  []CaddyMatch  `json:"match"`
	Handle []CaddyHandle `json:"handle"`
}

type CaddyRule struct {
	Listen []string     `json:"listen"`
	Routes []CaddyRoute `json:"routes"`
}

func createCaddyConfigWithHost(host string, upstreamAddress string, listenAddress string) CaddyRule {
	match := CaddyMatch{
		Host: []string{host},
	}
	upstream := CaddyUpstream{
		Dial: upstreamAddress,
	}
	handle := CaddyHandle{
		Handler:   "reverse_proxy",
		Upstreams: []CaddyUpstream{upstream},
	}
	return CaddyRule{
		Listen: []string{listenAddress},
		Routes: []CaddyRoute{
			{
				Match:  []CaddyMatch{match},
				Handle: []CaddyHandle{handle},
			},
		},
	}
}

type CaddyError struct {
	Error error
	Body  []byte
}

var client *http.Client = nil

func retryRequest(client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error
	for i := range maxRetries {
		slog.Info("Retry")
		resp, err = client.Do(req)
		if err == nil {
			return resp, nil
		} else {
			slog.Error("httpCaddyRequest retry failed", "error", err.Error())
			time.Sleep(time.Duration(1<<i) * time.Second) // Exponential backoff
		}
	}
	return nil, err
}

func initCaddyClient() {
	// Initialize the shared http.Client
	client = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func HttpCaddyRequest(url string, method string, data *string) (int, []byte, error) {
	if client == nil {
		initCaddyClient()
	}

	if data != nil {
		slog.Info("HttpCaddyRequest", "method", method, "url", url, "data", *data)
	} else {
		slog.Info("HttpCaddyRequest", "method", method, "url", url)
	}
	var reader io.Reader = nil
	if data != nil {
		reader = strings.NewReader(*data)
	} else {
		// TODO
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return -1, nil, errors.New(fmt.Sprintf("Error connecting to Caddy: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("HttpCaddyRequest failed", "error", err)
		req, err := http.NewRequest(method, url, reader)
		req.Header.Set("Content-Type", "application/json")

		resp, err = retryRequest(client, req, 3)
		if err != nil {
			return -1, nil, errors.New(fmt.Sprintf("Error sending request to Caddy: %v", err))
		}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	return resp.StatusCode, body, err
}

func DeleteReverseProxy(containerName string) CaddyError {
	slog.Info("DeleteReverseProxy", "container", containerName)

	localCaddyUrl := config.Get().Daemon.LocalCaddyUrl
	localCaddyUrl = fmt.Sprintf("http://%s/id/%s", localCaddyUrl, containerName)
	req, err := http.NewRequest("DELETE", localCaddyUrl, nil)
	if err != nil {
		return CaddyError{errors.New(fmt.Sprintf("Error connecting to Caddy: %v", err)), nil}
	}

	req.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return CaddyError{errors.New(fmt.Sprintf("Error sending request: %v\n", err)), nil}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		slog.Info("DeleteProxyHandler: Local proxy route deleted. Deleting master route", "container", containerName)

		if err := deleteMasterRoute(containerName); err != nil {
			slog.Error("Error on DeleteProxyHandler. Master route deletion", "container", containerName, "error", err.Error())
		} else {
			slog.Error("DeleteProxyHandler. Master route successfully deleted.", "container", containerName)
		}
		return CaddyError{nil, nil}
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return CaddyError{errors.New(fmt.Sprintf("Error reading body response: %v", err)), nil}
		} else {
			return CaddyError{errors.New(fmt.Sprintf("Caddy returned error: %v", resp.StatusCode)), body}
		}
	}
}

func SetReverseProxy(containerName string, opts ProxyOpts) CaddyError {
	slog.Info("SetReverseProxy", "container", containerName, "opts", opts)

	localCaddyUrl := config.Get().Daemon.LocalCaddyUrl
	localCaddyUrl = fmt.Sprintf("http://%s/config/apps/http/servers/realm/routes", localCaddyUrl)

	var caddyRequest string
	if opts.HttpsUpstream {
		caddyRequest = getHTTPRouteConfig(containerName, opts.Hosts, opts.Upstream, true)
	} else if opts.HttpUpstream {
		caddyRequest = getHTTPRouteConfig(containerName, opts.Hosts, opts.Upstream, false)
	} else {
		caddyRequest = getRouteConfig(containerName, opts.Hosts, opts.Upstream)
	}
	req, err := http.NewRequest("POST", localCaddyUrl, strings.NewReader(caddyRequest))
	if err != nil {
		return CaddyError{errors.New(fmt.Sprintf("Error connecting to Caddy: %v", err)), nil}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return CaddyError{errors.New(fmt.Sprintf("Error sending request: %v\n", err)), nil}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		slog.Info("SetReverseProxy. Local proxy route created. Creating master route", "container", containerName)

		upstream := fmt.Sprintf("%s:443", config.Get().Daemon.ListenAddress)
		isHTTP := opts.HttpUpstream || opts.HttpsUpstream
		if err := createMasterRoute(containerName, opts.Hosts, upstream, isHTTP); err != nil {
			slog.Error("Ignoring error on SetReverseProxy. Master route creation", "container", containerName, "error", err.Error())
		} else {
			slog.Info("SetReverseProxy. Master route successfully created", "container", containerName)
		}

		return CaddyError{nil, nil}
	} else {
		return CaddyError{errors.New(fmt.Sprintf("Error on SetReverseProxy: Caddy returned error %d %s for request %s. Continuing...", resp.StatusCode, body, caddyRequest)), body}
	}
}

func GetReverseProxyConfig(caddyID string) (int, []byte, error) {
	slog.Info("GetReverseProxyConfig", "caddyID", caddyID)

	localCaddyUrl := config.Get().Daemon.LocalCaddyUrl
	localCaddyUrl = fmt.Sprintf("http://%s/id/%s", localCaddyUrl, caddyID)

	req, err := http.NewRequest("GET", localCaddyUrl, nil)
	if err != nil {
		return -1, nil, errors.New(fmt.Sprintf("Error connecting to Caddy: %v\n", err))
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -1, nil, errors.New(fmt.Sprintf("Error sending request: %v\n", err))
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, nil, errors.New(fmt.Sprintf("Error readng response: %v\n", err))
	}

	return resp.StatusCode, body, nil
}

func AddProxyHost(caddyID string, domain string) error {
	// Local caddy
	{
		localCaddyURL := config.Get().Daemon.LocalCaddyUrl
		localCaddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host", localCaddyURL, caddyID)

		caddyRequest := fmt.Sprintf(`"%s"`, domain)
		statusCode, body, err := HttpCaddyRequest(localCaddyURL, "POST", &caddyRequest)
		if err != nil {
			return err
		}

		isDuplicatedEntry := false
		if statusCode == 500 && body != nil && bytes.Contains(body, []byte("repeated")) {
			isDuplicatedEntry = true
		} else {
			slog.Info("AddProxyHost duplicated entry", "caddyID", caddyID, "domain", domain)
		}

		if statusCode != 200 && !isDuplicatedEntry {
			return errors.New(fmt.Sprintf("Invalid local caddy statuscode %d: %s", statusCode, body))
		}
	}
	// Master caddy
	{
		masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
		masterCaddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host", masterCaddyURL, caddyID)

		caddyRequest := fmt.Sprintf(`"%s"`, domain)
		statusCode, body, err := HttpCaddyRequest(masterCaddyURL, "POST", &caddyRequest)
		if err != nil {
			return err
		}
		isDuplicatedEntry := false
		if statusCode == 500 && body != nil && bytes.Contains(body, []byte("repeated")) {
			isDuplicatedEntry = true
		} else {
			slog.Info("AddProxyHost duplicated entry", "caddyID", caddyID, "domain", domain)
		}

		if statusCode != 200 && !isDuplicatedEntry {
			return errors.New(fmt.Sprintf("Invalid local caddy statuscode %d: %s", statusCode, body))
		}
	}
	return nil
}

func GetMasterProxyHosts(caddyID string) ([]string, error) {
	var masterHosts []string

	masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
	masterCaddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host", masterCaddyURL, caddyID)
	statusCode, masterBody, err := HttpCaddyRequest(masterCaddyURL, "GET", nil)
	if err != nil {
		return nil, err
	}
	if statusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Invalid caddy statuscode %d", statusCode))
	}
	err = json.Unmarshal([]byte(masterBody), &masterHosts)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error unmarshalling JSON: %v", err))
	}

	return masterHosts, nil
}

func GetProxyHosts(caddyID string) ([]string, []string, error) {
	var localHosts []string
	var masterHosts []string

	localCaddyURL := config.Get().Daemon.LocalCaddyUrl
	localCaddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host", localCaddyURL, caddyID)
	statusCode, localBody, err := HttpCaddyRequest(localCaddyURL, "GET", nil)
	if err != nil {
		return nil, nil, err
	}
	if statusCode != 200 {
		return nil, nil, errors.New(fmt.Sprintf("Invalid caddy statuscode %d", statusCode))
	}
	err = json.Unmarshal([]byte(localBody), &localHosts)
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling JSON: %v", err))
	}

	masterHosts, err = GetMasterProxyHosts(caddyID)
	if err != nil {
		return nil, nil, err
	}

	return localHosts, masterHosts, nil
}

func RemoveProxyHost(caddyID string, domain string) error {
	localHosts, masterHosts, err := GetProxyHosts(caddyID)
	if err != nil {
		return err
	}
	localHostsIndex := -1
	for index, host := range localHosts {
		if host == domain {
			localHostsIndex = index
			break
		}
	}
	masterHostsIndex := -1
	for index, host := range masterHosts {
		if host == domain {
			masterHostsIndex = index
			break
		}
	}

	if localHostsIndex != -1 {
		caddyURL := config.Get().Daemon.LocalCaddyUrl
		caddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host/%d", caddyURL, caddyID, localHostsIndex)
		HttpCaddyRequest(caddyURL, "DELETE", nil)
	}

	if masterHostsIndex != -1 {
		masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
		masterCaddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host/%d", masterCaddyURL, caddyID, masterHostsIndex)
		HttpCaddyRequest(masterCaddyURL, "DELETE", nil)
	}

	return nil
}
