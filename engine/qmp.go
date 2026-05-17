package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// QMPClient is a minimal QEMU Machine Protocol client over a Unix socket.
// The QMP protocol is newline-delimited JSON. On connect the server sends a
// greeting; the client responds with qmp_capabilities and is then ready to
// send commands.
type QMPClient struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

type qmpCommand struct {
	Execute string      `json:"execute"`
	Args    interface{} `json:"arguments,omitempty"`
}

type qmpResponse struct {
	Return json.RawMessage `json:"return"`
	Error  *qmpError       `json:"error,omitempty"`
	Event  string          `json:"event,omitempty"`
}

type qmpError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// connectQMP opens a QMP session to the VM's monitor socket.
func (v *VirtualMachine) connectQMP() (*QMPClient, error) {
	monPath := v.monitorSocketPath()
	conn, err := net.DialTimeout("unix", monPath, 3*time.Second)
	if err != nil {
		return nil, ErrQMPNotAvailable
	}

	client := &QMPClient{
		conn:    conn,
		scanner: bufio.NewScanner(conn),
	}

	// Read the QMP greeting
	if _, err := client.readResponse(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: failed to read greeting: %w", err)
	}

	// Perform capability negotiation
	if err := client.execute("qmp_capabilities", nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: capability negotiation failed: %w", err)
	}

	return client, nil
}

func (c *QMPClient) Close() error {
	return c.conn.Close()
}

// readResponse reads one JSON line from the QMP socket.
func (c *QMPClient) readResponse() (*qmpResponse, error) {
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("qmp: connection closed")
	}

	var resp qmpResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("qmp: failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("qmp: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return &resp, nil
}

// execute sends a QMP command and reads the response.
func (c *QMPClient) execute(cmd string, args interface{}) error {
	payload, err := json.Marshal(qmpCommand{Execute: cmd, Args: args})
	if err != nil {
		return err
	}

	payload = append(payload, '\n')
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write(payload); err != nil {
		return err
	}

	// Skip event messages until we get the command response
	for {
		resp, err := c.readResponse()
		if err != nil {
			return err
		}
		if resp.Event == "" {
			return nil
		}
		// It was an async event; loop to get the real response
	}
}

// executeWithResult sends a QMP command and returns the raw JSON return value.
func (c *QMPClient) executeWithResult(cmd string, args interface{}) (json.RawMessage, error) {
	payload, err := json.Marshal(qmpCommand{Execute: cmd, Args: args})
	if err != nil {
		return nil, err
	}

	payload = append(payload, '\n')
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write(payload); err != nil {
		return nil, err
	}

	for {
		resp, err := c.readResponse()
		if err != nil {
			return nil, err
		}
		if resp.Event == "" {
			return resp.Return, nil
		}
	}
}

// PowerDown sends a graceful ACPI shutdown signal to the guest OS.
func (c *QMPClient) PowerDown() error {
	return c.execute("system_powerdown", nil)
}

// ── Stats queries ─────────────────────────────────────────────────────────────

type QMPStatus struct {
	Running    bool   `json:"running"`
	Status     string `json:"status"`
	Singlestep bool   `json:"singlestep"`
}

type QMPBalloon struct {
	Actual int64 `json:"actual"`
}

type QMPBlockDevice struct {
	Device   string          `json:"device"`
	Inserted *QMPBlockInsert `json:"inserted"`
}

type QMPBlockInsert struct {
	Image   QMPBlockImage `json:"image"`
	RdBytes int64         `json:"rd_bytes"`
	WrBytes int64         `json:"wr_bytes"`
	RdOps   int64         `json:"rd_operations"`
	WrOps   int64         `json:"wr_operations"`
}

type QMPBlockImage struct {
	Filename string `json:"filename"`
	Format   string `json:"format"`
}

func (c *QMPClient) QueryStatus() (*QMPStatus, error) {
	raw, err := c.executeWithResult("query-status", nil)
	if err != nil {
		return nil, err
	}
	var status QMPStatus
	return &status, json.Unmarshal(raw, &status)
}

func (c *QMPClient) QueryBalloon() (*QMPBalloon, error) {
	raw, err := c.executeWithResult("query-balloon", nil)
	if err != nil {
		return nil, err
	}
	var balloon QMPBalloon
	return &balloon, json.Unmarshal(raw, &balloon)
}

func (c *QMPClient) QueryBlock() ([]QMPBlockDevice, error) {
	raw, err := c.executeWithResult("query-block", nil)
	if err != nil {
		return nil, err
	}
	var devices []QMPBlockDevice
	return devices, json.Unmarshal(raw, &devices)
}

// SetBalloon adjusts the VM's memory balloon target to the given value in bytes.
// The virtio-balloon-pci device must be present (it is added by default).
func (c *QMPClient) SetBalloon(bytes int64) error {
	return c.execute("balloon", map[string]interface{}{"value": bytes})
}

// Pause suspends the VM's CPU (QMP 'stop'). The VM process stays alive.
func (c *QMPClient) Pause() error {
	return c.execute("stop", nil)
}

// Resume resumes CPU execution on a paused VM (QMP 'cont').
func (c *QMPClient) Resume() error {
	return c.execute("cont", nil)
}
