//go:build linux

package microvm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type FCDrive struct {
	File     string `json:"file,omitempty"`
	ID       string `json:"id,omitempty"`
	Root     bool   `json:"root,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Resize   string `json:"resize,omitempty"`
}

type FCNetdev struct {
	ID  string `json:"id,omitempty"`
	Tap string `json:"tap,omitempty"`
	BR  string `json:"br,omitempty"`
	Mac string `json:"mac,omitempty"`
}

type FCVsock struct {
	CID uint32 `json:"cid,omitempty"`
	UDS string `json:"uds,omitempty"`
}

type FCBalloon struct {
	AmountMiB             int  `json:"amount_mib,omitempty"`
	DeflateOnOOM          bool `json:"deflate_on_oom,omitempty"`
	StatsPollingIntervalS int  `json:"stats_polling_interval_s,omitempty"`
}

type FCConfig struct {
	Binary    string     `json:"binary,omitempty"`
	Kernel    string     `json:"kernel,omitempty"`
	Initrd    string     `json:"initrd,omitempty"`
	BootArgs  string     `json:"boot_args,omitempty"`
	Memory    int        `json:"memory,omitempty"`
	VCPUs     int        `json:"vcpus,omitempty"`
	SMT       bool       `json:"smt,omitempty"`
	Drives    []FCDrive  `json:"drives,omitempty"`
	Netdev    []FCNetdev `json:"netdev,omitempty"`
	Vsock     *FCVsock   `json:"vsock,omitempty"`
	Balloon   *FCBalloon `json:"balloon,omitempty"`
	APISocket string     `json:"api_socket,omitempty"`
}

type FCNodeMetadata struct {
	PID        int                  `json:"pid"`
	APISocket  string               `json:"api_socket"`
	KernelPath string               `json:"kernel_path"`
	InitrdPath string               `json:"initrd_path,omitempty"`
	BootPlan   BootPlan             `json:"boot_plan"`
	Drives     map[int]OverlayImage `json:"overlay_drives"`
}

type fcInstanceInfo struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	VMMVersion string `json:"vmm_version"`
}

type fcMachineConfig struct {
	VCPUCount  int  `json:"vcpu_count"`
	MemSizeMiB int  `json:"mem_size_mib"`
	SMT        bool `json:"smt,omitempty"`
}

type fcBootSource struct {
	KernelImagePath string `json:"kernel_image_path"`
	InitrdPath      string `json:"initrd_path,omitempty"`
	BootArgs        string `json:"boot_args,omitempty"`
}

type fcDrive struct {
	DriveID      string `json:"drive_id"`
	PathOnHost   string `json:"path_on_host"`
	IsRootDevice bool   `json:"is_root_device"`
	IsReadOnly   bool   `json:"is_read_only"`
}

type fcNetworkInterface struct {
	IfaceID     string `json:"iface_id"`
	HostDevName string `json:"host_dev_name"`
	GuestMAC    string `json:"guest_mac,omitempty"`
}

type fcBalloon struct {
	AmountMiB             int  `json:"amount_mib"`
	DeflateOnOOM          bool `json:"deflate_on_oom"`
	StatsPollingIntervalS int  `json:"stats_polling_interval_s,omitempty"`
}

type fcAction struct {
	ActionType string `json:"action_type"`
}

var apiClients sync.Map // socket path -> *http.Client

func apiClient(socket string) *http.Client {
	if c, ok := apiClients.Load(socket); ok {
		return c.(*http.Client)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
			MaxIdleConnsPerHost: 1,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	actual, loaded := apiClients.LoadOrStore(socket, client)
	if loaded {
		client.CloseIdleConnections()
	}
	return actual.(*http.Client)
}

func apiDo(socket, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, "http://localhost"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := apiClient(socket).Do(req)
	if err != nil {
		return fmt.Errorf("firecracker: %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("firecracker: %s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("firecracker: %s %s: invalid response: %w", method, path, err)
		}
	}
	return nil
}

func instanceInfo(socket string) (fcInstanceInfo, error) {
	var info fcInstanceInfo
	err := apiDo(socket, http.MethodGet, "/", nil, &info)
	return info, err
}

func sendAction(socket, action string) error {
	return apiDo(socket, http.MethodPut, "/actions", fcAction{ActionType: action}, nil)
}

func waitAPIReady(socket string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := instanceInfo(socket); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("firecracker: API socket %s did not become ready: %w", socket, lastErr)
}
