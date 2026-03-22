package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Pipe/socket paths for tscode center IPC.
const (
	tscodePipeWindows = `\\.\pipe\terminal-studio-code`
	tscodePipeUnix    = "/tmp/tscode.sock"
)

// tscodeTimeout is the maximum time to wait for a tscode IPC response.
const tscodeTimeout = 5 * time.Second

// ---------------------------------------------------------------------------
// Wire types — match the tscode center JSON protocol.
// ---------------------------------------------------------------------------

type tscodeRequest struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type tscodeResponse struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type openSessionData struct {
	SessionID      string `json:"session_id,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	Command        string `json:"command,omitempty"`
	AddToTab       int    `json:"add_to_tab,omitempty"`
	SplitDirection string `json:"split_direction,omitempty"`
}

// TSCodeOpenResult holds the response from an open_session request.
type TSCodeOpenResult struct {
	TabID     int `json:"tab_id"`
	SessionID int `json:"session_id"`
}

// TSCodeTabInfo describes a single tab in the running tscode center instance.
type TSCodeTabInfo struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	PaneCount int    `json:"pane_count"`
	IsActive  bool   `json:"is_active"`
}

type listTabsResponse struct {
	Tabs []TSCodeTabInfo `json:"tabs"`
}

// ---------------------------------------------------------------------------
// Exported IPC functions
// ---------------------------------------------------------------------------

// TSCodeIsRunning checks whether a tscode center process is listening.
func TSCodeIsRunning() bool {
	conn, err := dialTSCode()
	if err != nil {
		return false
	}
	defer conn.Close()

	resp, err := tscodeRoundTrip(conn, tscodeRequest{Type: "ping"})
	if err != nil {
		return false
	}
	return resp.OK
}

// TSCodeListTabs returns the current tabs in the running tscode center.
func TSCodeListTabs() ([]TSCodeTabInfo, error) {
	conn, err := dialTSCode()
	if err != nil {
		return nil, fmt.Errorf("tscode not running: %w", err)
	}
	defer conn.Close()

	resp, err := tscodeRoundTrip(conn, tscodeRequest{Type: "list_tabs"})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("tscode error: %s", resp.Error)
	}
	var tabs listTabsResponse
	if err := json.Unmarshal(resp.Data, &tabs); err != nil {
		return nil, fmt.Errorf("parsing tabs response: %w", err)
	}
	return tabs.Tabs, nil
}

// TSCodeOpenSession sends an open_session request to the tscode center.
func TSCodeOpenSession(sessionID, cwd, command string, addToTab int, splitDir string) (*TSCodeOpenResult, error) {
	conn, err := dialTSCode()
	if err != nil {
		return nil, fmt.Errorf("tscode not running: %w", err)
	}
	defer conn.Close()

	data := openSessionData{
		SessionID:      sessionID,
		CWD:            cwd,
		Command:        command,
		AddToTab:       addToTab,
		SplitDirection: splitDir,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshalling session data: %w", err)
	}

	resp, err := tscodeRoundTrip(conn, tscodeRequest{Type: "open_session", Data: dataBytes})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("tscode error: %s", resp.Error)
	}
	var result TSCodeOpenResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parsing open_session response: %w", err)
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// tscodeRoundTrip sends a JSON request and reads a JSON response with a
// timeout. It uses a goroutine+channel pattern because the underlying
// connection may be an *os.File (Windows named pipe) which does not
// support Set{Read,Write}Deadline.
func tscodeRoundTrip(conn io.ReadWriteCloser, req tscodeRequest) (*tscodeResponse, error) {
	type result struct {
		resp *tscodeResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		if err := json.NewEncoder(conn).Encode(req); err != nil {
			ch <- result{nil, fmt.Errorf("writing to tscode: %w", err)}
			return
		}
		var resp tscodeResponse
		if err := json.NewDecoder(conn).Decode(&resp); err != nil {
			ch <- result{nil, fmt.Errorf("reading from tscode: %w", err)}
			return
		}
		ch <- result{&resp, nil}
	}()

	select {
	case r := <-ch:
		return r.resp, r.err
	case <-time.After(tscodeTimeout):
		return nil, errors.New("tscode IPC timeout")
	}
}
