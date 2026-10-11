package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

const wait = 5 * time.Second

func noEnv(string) string { return "" }

// cancelled returns a context that is already cancelled. A run that should
// fail before listening gets it, so a bug that makes it serve cannot hang the
// test: run then shuts down at once and returns nil.
func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// testServer is a run started by a test.
type testServer struct {
	addr   string
	cancel context.CancelFunc
	done   <-chan error  // the result of run
	msgs   <-chan string // the message of each log line, in order
}

// startServer starts run with args and waits until it listens.
func startServer(t *testing.T, args ...string) *testServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	logR, logW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, args, noEnv, logW)
		logW.Close()
	}()

	msgs := make(chan string, 16)
	addrs := make(chan string, 1)
	go func() {
		defer close(msgs)
		lines := bufio.NewScanner(logR)
		for lines.Scan() {
			var line struct {
				Msg  string `json:"msg"`
				Addr string `json:"addr"`
			}
			if err := json.Unmarshal(lines.Bytes(), &line); err != nil {
				line.Msg = "not JSON: " + lines.Text()
			}
			if line.Msg == "listening" {
				addrs <- line.Addr
			}
			msgs <- line.Msg
		}
	}()

	s := &testServer{cancel: cancel, done: done, msgs: msgs}
	select {
	case s.addr = <-addrs:
	case err := <-done:
		t.Fatalf("run returned before listening: %v", err)
	case <-time.After(wait):
		t.Fatal("run did not start listening")
	}
	return s
}

// stop cancels run's context and returns the result of run.
func (s *testServer) stop(t *testing.T) error {
	t.Helper()
	s.cancel()
	select {
	case err := <-s.done:
		return err
	case <-time.After(wait):
		t.Fatal("run did not return after its context was cancelled")
		return nil
	}
}

// waitLog waits for a log line with the message want.
func (s *testServer) waitLog(t *testing.T, want string) {
	t.Helper()
	for {
		select {
		case msg, ok := <-s.msgs:
			if !ok {
				t.Fatalf("the log ended before %q", want)
			}
			if msg == want {
				return
			}
		case <-time.After(wait):
			t.Fatalf("no log line %q", want)
		}
	}
}

func get(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: wait}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}

func TestRun(t *testing.T) {
	s := startServer(t, "-addr", "127.0.0.1:0")
	base := "http://" + s.addr

	status, body := get(t, http.MethodPost, base+"/api/v1/calculate", `{"expression": "2 * (3 + 4) ^ 2"}`)
	if status != http.StatusOK || body != `{"result":"98"}` {
		t.Errorf("POST /api/v1/calculate = %d %s, want 200 {\"result\":\"98\"}", status, body)
	}
	// The API is mounted under /api/, and nothing else is served yet.
	for _, tt := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/api/v1/calculate", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/v1/other", http.StatusNotFound},
		{http.MethodGet, "/", http.StatusNotFound},
	} {
		if status, _ := get(t, tt.method, base+tt.path, ""); status != tt.status {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, status, tt.status)
		}
	}

	// Cancelling the context shuts the server down, and run returns nil.
	if err := s.stop(t); err != nil {
		t.Fatalf("run returned %v after its context was cancelled, want nil", err)
	}
	s.waitLog(t, "shutting down")
	s.waitLog(t, "stopped")
	if conn, err := net.DialTimeout("tcp", s.addr, time.Second); err == nil {
		conn.Close()
		t.Error("the server still accepts connections after run returned")
	}
}

// A request in flight when the context is cancelled still gets its response.
func TestGracefulShutdown(t *testing.T) {
	s := startServer(t, "-addr", "127.0.0.1:0")
	conn, err := net.DialTimeout("tcp", s.addr, wait)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(wait))

	// Send only the headers. The server answers "100 Continue" when the
	// handler starts reading the body, so the request is then in flight.
	body := `{"expression": "1 + 2"}`
	fmt.Fprintf(conn, "POST /api/v1/calculate HTTP/1.1\r\nHost: test\r\nContent-Length: %d\r\nExpect: 100-continue\r\n\r\n", len(body))
	replies := bufio.NewReader(conn)
	resp, err := http.ReadResponse(replies, nil)
	if err != nil || resp.StatusCode != http.StatusContinue {
		t.Fatalf("first response = %v, %v; want 100 Continue", resp, err)
	}

	// Shut down. Shutdown closes the listener first, so once new connections
	// are refused, it is waiting for the request.
	s.cancel()
	for deadline := time.Now().Add(wait); ; time.Sleep(time.Millisecond) {
		c, err := net.DialTimeout("tcp", s.addr, wait)
		if err != nil {
			break
		}
		c.Close()
		if time.Now().After(deadline) {
			t.Fatal("the server still accepts connections after its context was cancelled")
		}
	}
	fmt.Fprint(conn, body)
	resp, err = http.ReadResponse(replies, nil)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(got) != `{"result":"3"}` {
		t.Errorf("response = %d %s, want 200 {\"result\":\"3\"}", resp.StatusCode, got)
	}
	if err := s.stop(t); err != nil {
		t.Errorf("run returned %v, want nil", err)
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"-port", "8080"}, "flag provided but not defined: -port"},
		{"extra argument", []string{"serve"}, `unexpected arguments: ["serve"]`},
		{"empty address", []string{"-addr", ""}, "-addr must not be empty"},
		{"invalid port", []string{"-addr", "127.0.0.1:99999"}, "invalid port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			err := run(cancelled(), tt.args, noEnv, &stderr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("run(%q) = %v, want an error containing %q", tt.args, err, tt.want)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	var stderr bytes.Buffer
	if err := run(cancelled(), []string{"-h"}, noEnv, &stderr); err != nil {
		t.Errorf("run(-h) = %v, want nil", err)
	}
	if !strings.Contains(stderr.String(), "-addr address") {
		t.Errorf("usage = %q, want it to describe -addr", stderr.String())
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{"default", nil, "", ":8080"},
		{"environment", nil, ":9000", ":9000"},
		{"flag", []string{"-addr", "127.0.0.1:7000"}, "", "127.0.0.1:7000"},
		{"flag over environment", []string{"-addr", "127.0.0.1:7000"}, ":9000", "127.0.0.1:7000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "ADDR" {
					return tt.env
				}
				return ""
			}
			cfg, err := parseConfig(tt.args, getenv, io.Discard)
			if err != nil || cfg.addr != tt.want {
				t.Errorf("parseConfig = %+v, %v; want addr %s", cfg, err, tt.want)
			}
		})
	}
}

// Ctrl+C and docker stop both shut the server down gracefully. Windows cannot
// send SIGTERM to a process, so the signal itself is checked with docker stop
// (PLAN.md step 17).
func TestShutdownSignals(t *testing.T) {
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		if !slices.Contains(shutdownSignals, sig) {
			t.Errorf("shutdownSignals = %v, want it to include %v", shutdownSignals, sig)
		}
	}
}
