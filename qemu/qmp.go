package qemu

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// QMPClient is a minimal QEMU Machine Protocol client over a Unix socket.
// It speaks the QMP wire format: JSON lines, capability handshake on connect.
type QMPClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

// NewQMPClient connects to a QEMU monitor socket and performs the capability
// negotiation required before any commands can be sent.
func NewQMPClient(socketPath string) (*QMPClient, error) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("qmp: dial %s: %w", socketPath, err)
	}

	c := &QMPClient{conn: conn, reader: bufio.NewReader(conn)}

	// Read and discard the greeting banner.
	if _, err := c.readResponse(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: read greeting: %w", err)
	}

	// Unlock the command set.
	if _, err := c.execute("qmp_capabilities", nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: capabilities: %w", err)
	}

	return c, nil
}

func (c *QMPClient) Close() error { return c.conn.Close() }

func (c *QMPClient) readResponse() (map[string]interface{}, error) {
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(line, &result); err != nil {
		return nil, fmt.Errorf("qmp: unmarshal %q: %w", line, err)
	}

	// Surface QMP-level errors.
	if errObj, ok := result["error"]; ok {
		if errMap, ok := errObj.(map[string]interface{}); ok {
			return nil, fmt.Errorf("qmp: %s: %s", errMap["class"], errMap["desc"])
		}
		return nil, fmt.Errorf("qmp error: %v", errObj)
	}

	return result, nil
}

func (c *QMPClient) execute(cmd string, args interface{}) (map[string]interface{}, error) {
	req := map[string]interface{}{"execute": cmd}
	if args != nil {
		req["arguments"] = args
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(c.conn, "%s\n", data); err != nil {
		return nil, err
	}

	// Skip async events (lines with "event" key) until we get a "return".
	for {
		resp, err := c.readResponse()
		if err != nil {
			return nil, err
		}
		if _, hasEvent := resp["event"]; hasEvent {
			continue
		}
		return resp, nil
	}
}

// SystemPowerdown sends an ACPI power-down request to the VM.
// This is equivalent to pressing the power button — the guest OS shuts down
// gracefully. Use with a timeout and fall back to SIGKILL if needed.
func (c *QMPClient) SystemPowerdown() error {
	_, err := c.execute("system_powerdown", nil)
	return err
}

// QueryBlock returns raw block device info from the VM.
func (c *QMPClient) QueryBlock() ([]interface{}, error) {
	resp, err := c.execute("query-block", nil)
	if err != nil {
		return nil, err
	}

	ret, ok := resp["return"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("qmp: query-block: unexpected return type")
	}
	return ret, nil
}

// QueryBalloon returns the current balloon device memory target in bytes.
func (c *QMPClient) QueryBalloon() (int64, error) {
	resp, err := c.execute("query-balloon", nil)
	if err != nil {
		return 0, err
	}

	ret, ok := resp["return"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("qmp: query-balloon: unexpected return type")
	}

	actual, ok := ret["actual"].(float64)
	if !ok {
		return 0, fmt.Errorf("qmp: query-balloon: missing actual field")
	}
	return int64(actual), nil
}
