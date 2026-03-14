package nodes

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"time"
)

const qmpDialTimeout = 5 * time.Second
const qmpReadTimeout = 5 * time.Second

// qmpConnect connects to the QMP TCP port, reads the greeting,
// and sends qmp_capabilities to enter command mode.
func qmpConnect(port int) (net.Conn, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, qmpDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("qmp: failed to connect to %s: %w", addr, err)
	}

	// Read greeting
	if err := conn.SetReadDeadline(time.Now().Add(qmpReadTimeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: failed to set read deadline: %w", err)
	}

	decoder := json.NewDecoder(conn)
	var greeting map[string]any
	if err := decoder.Decode(&greeting); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: failed to read greeting: %w", err)
	}

	// Send qmp_capabilities to enter command mode
	capCmd := map[string]string{"execute": "qmp_capabilities"}
	if err := json.NewEncoder(conn).Encode(capCmd); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: failed to send qmp_capabilities: %w", err)
	}

	// Read qmp_capabilities response
	if err := conn.SetReadDeadline(time.Now().Add(qmpReadTimeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: failed to set read deadline: %w", err)
	}
	var capResp map[string]any
	if err := decoder.Decode(&capResp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: failed to read qmp_capabilities response: %w", err)
	}

	return conn, nil
}

// qmpExecCommand connects to the QMP TCP port, executes a command,
// skips any event messages, and returns the command response.
func qmpExecCommand(port int, command string) (map[string]any, error) {
	conn, err := qmpConnect(port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Send command
	cmd := map[string]string{"execute": command}
	if err := json.NewEncoder(conn).Encode(cmd); err != nil {
		return nil, fmt.Errorf("qmp: failed to send command %s: %w", command, err)
	}

	// Read response, skipping event messages
	decoder := json.NewDecoder(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(qmpReadTimeout)); err != nil {
			return nil, fmt.Errorf("qmp: failed to set read deadline: %w", err)
		}
		var resp map[string]any
		if err := decoder.Decode(&resp); err != nil {
			return nil, fmt.Errorf("qmp: failed to read response for %s: %w", command, err)
		}
		// Skip event messages
		if _, isEvent := resp["event"]; isEvent {
			continue
		}
		return resp, nil
	}
}

func qmpSystemPowerdown(port int) error {
	resp, err := qmpExecCommand(port, "system_powerdown")
	if err != nil {
		return err
	}
	if _, ok := resp["return"]; !ok {
		return fmt.Errorf("qmp: system_powerdown failed: %v", resp)
	}
	return nil
}

func qmpSystemReset(port int) error {
	resp, err := qmpExecCommand(port, "system_reset")
	if err != nil {
		return err
	}
	if _, ok := resp["return"]; !ok {
		return fmt.Errorf("qmp: system_reset failed: %v", resp)
	}
	return nil
}

func qmpQuit(port int) error {
	resp, err := qmpExecCommand(port, "quit")
	if err != nil {
		return err
	}
	if _, ok := resp["return"]; !ok {
		return fmt.Errorf("qmp: quit failed: %v", resp)
	}
	return nil
}

// qmpQueryBalloon queries the balloon device and returns the actual memory in bytes.
// Requires the virtio-balloon device to be configured in the VM.
func qmpQueryBalloon(port int) (uint64, error) {
	resp, err := qmpExecCommand(port, "query-balloon")
	if err != nil {
		return 0, err
	}
	ret, ok := resp["return"]
	if !ok {
		return 0, fmt.Errorf("qmp: query-balloon failed: %v", resp)
	}
	retMap, ok := ret.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("qmp: unexpected query-balloon return type: %v", ret)
	}
	actual, ok := retMap["actual"].(float64)
	if !ok {
		return 0, fmt.Errorf("qmp: unexpected actual field type: %v", retMap["actual"])
	}
	return uint64(actual), nil
}

// qmpQueryMemorySizeSummary queries the total memory size and returns it in bytes.
func qmpQueryMemorySizeSummary(port int) (uint64, error) {
	resp, err := qmpExecCommand(port, "query-memory-size-summary")
	if err != nil {
		return 0, err
	}
	ret, ok := resp["return"]
	if !ok {
		return 0, fmt.Errorf("qmp: query-memory-size-summary failed: %v", resp)
	}
	retMap, ok := ret.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("qmp: unexpected query-memory-size-summary return type: %v", ret)
	}
	baseMem, ok := retMap["base-memory"].(float64)
	if !ok {
		return 0, fmt.Errorf("qmp: unexpected base-memory field type: %v", retMap["base-memory"])
	}
	pluggedMem, _ := retMap["plugged-memory"].(float64)
	return uint64(baseMem) + uint64(pluggedMem), nil
}

// qmpQueryCpusFast queries the number of vCPUs.
func qmpQueryCpusFast(port int) (int, error) {
	resp, err := qmpExecCommand(port, "query-cpus-fast")
	if err != nil {
		return 0, err
	}
	ret, ok := resp["return"]
	if !ok {
		return 0, fmt.Errorf("qmp: query-cpus-fast failed: %v", resp)
	}
	cpuList, ok := ret.([]any)
	if !ok {
		return 0, fmt.Errorf("qmp: unexpected query-cpus-fast return type: %v", ret)
	}
	return len(cpuList), nil
}

// qmpStop sends the 'stop' command to pause a running VM's CPU.
func qmpStop(port int) error {
	resp, err := qmpExecCommand(port, "stop")
	if err != nil {
		return err
	}
	if _, ok := resp["return"]; !ok {
		return fmt.Errorf("qmp: stop failed: %v", resp)
	}
	return nil
}

// qmpCont sends the 'cont' command to resume a paused VM.
func qmpCont(port int) error {
	resp, err := qmpExecCommand(port, "cont")
	if err != nil {
		return err
	}
	if _, ok := resp["return"]; !ok {
		return fmt.Errorf("qmp: cont failed: %v", resp)
	}
	return nil
}

// qmpQueryStatus queries the VM status and returns whether the VM is running.
func qmpQueryStatus(port int) (bool, error) {
	resp, err := qmpExecCommand(port, "query-status")
	if err != nil {
		return false, err
	}
	ret, ok := resp["return"]
	if !ok {
		return false, fmt.Errorf("qmp: query-status failed: %v", resp)
	}
	retMap, ok := ret.(map[string]any)
	if !ok {
		return false, fmt.Errorf("qmp: unexpected query-status return type: %v", ret)
	}
	running, ok := retMap["running"].(bool)
	if !ok {
		return false, fmt.Errorf("qmp: unexpected running field type: %v", retMap["running"])
	}
	return running, nil
}

func waitForQMP(qmpPort int, cmd *exec.Cmd) error {
	procDone := make(chan error, 1)
	go func() {
		procDone <- cmd.Wait()
	}()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(30 * time.Second)
	for {
		select {
		case err := <-procDone:
			if err != nil {
				return fmt.Errorf("qemu process exited with error: %w", err)
			}
			return fmt.Errorf("qemu process exited unexpectedly")
		case <-deadline:
			return fmt.Errorf("timed out waiting for QMP to become available on port %d", qmpPort)
		case <-ticker.C:
			_, err := qmpQueryStatus(qmpPort)
			if err == nil {
				return nil
			}
		}
	}
}

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
