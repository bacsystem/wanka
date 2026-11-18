package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wanka-bin")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "wanka")
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const secretURL = "postgres://u:TOP-SECRET-PW@localhost/x"

// proc is a running wanka binary.
type proc struct {
	cmd    *exec.Cmd
	lines  chan map[string]any
	stderr *strings.Builder
}

func start(t *testing.T, env []string, args ...string) *proc {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &proc{cmd: cmd, lines: make(chan map[string]any, 64), stderr: &strings.Builder{}}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.lines)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				p.lines <- m
			}
		}
	}()
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return p
}

// waitMsg returns the first log line with the given msg.
func (p *proc) waitMsg(t *testing.T, msg string) map[string]any {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m, ok := <-p.lines:
			if !ok {
				t.Fatalf("process ended before %q; stderr: %s", msg, p.stderr)
			}
			if m["msg"] == msg {
				return m
			}
		case <-timeout:
			t.Fatalf("timeout waiting for %q", msg)
		}
	}
}

// terminate sends SIGTERM and requires a clean exit (code 0).
func (p *proc) terminate(t *testing.T) {
	t.Helper()
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("did not exit cleanly: %v; stderr: %s", err, p.stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("process did not exit after SIGTERM")
	}
	for range p.lines { // drain so the reader goroutine ends
	}
}

func okEnv() []string { return []string{"DATABASE_URL=" + secretURL, "HTTP_ADDR=127.0.0.1:0"} }

func TestServePlanesStartAndStopOnSIGTERM(t *testing.T) {
	for _, plane := range []string{"tenant", "platform", "webhooks"} {
		t.Run(plane, func(t *testing.T) {
			p := start(t, okEnv(), "serve", "--plane="+plane)
			l := p.waitMsg(t, "listening")
			addr, _ := l["addr"].(string)
			if l["plane"] != plane || addr == "" {
				t.Fatalf("unexpected listening line: %v", l)
			}
			resp, err := http.Get("http://" + addr + "/healthz")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("healthz %d", resp.StatusCode)
			}
			resp, err = http.Get("http://" + addr + "/readyz")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 503 {
				t.Fatalf("readyz %d", resp.StatusCode)
			}
			p.terminate(t)
		})
	}
}

func TestWorkerAndMigrateStopOnSIGTERM(t *testing.T) {
	for _, sub := range []string{"worker", "migrate"} {
		t.Run(sub, func(t *testing.T) {
			p := start(t, okEnv(), sub)
			p.waitMsg(t, "started")
			p.terminate(t)
		})
	}
}

func TestBadInvocationExitsNonZero(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, "usage"},
		{"unknown subcommand", []string{"frobnicate"}, "frobnicate"},
		{"unknown plane", []string{"serve", "--plane=nope"}, "nope"},
		{"missing plane", []string{"serve"}, "--plane"},
		{"unknown flag", []string{"serve", "--plane=tenant", "--bogus"}, "bogus"},
		{"extra args", []string{"worker", "extra"}, "extra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tc.args...)
			cmd.Env = okEnv()
			out, err := cmd.CombinedOutput()
			var ee *exec.ExitError
			if !asExit(err, &ee) || ee.ExitCode() == 0 {
				t.Fatalf("want non-zero exit, got %v", err)
			}
			if !strings.Contains(strings.ToLower(string(out)), strings.ToLower(tc.want)) {
				t.Fatalf("output %q should mention %q", out, tc.want)
			}
		})
	}
}

func asExit(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	*target = ee
	return ok
}

func TestMissingRequiredVariableIsNamedNotValued(t *testing.T) {
	for _, sub := range [][]string{{"serve", "--plane=tenant"}, {"worker"}, {"migrate"}} {
		cmd := exec.Command(binPath, sub...)
		cmd.Env = []string{"HTTP_ADDR=127.0.0.1:0", "LOG_LEVEL=" + secretURL}
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("%v: expected failure", sub)
		}
		if !strings.Contains(string(out), "DATABASE_URL") {
			t.Fatalf("%v: output %q does not name DATABASE_URL", sub, out)
		}
		if strings.Contains(string(out), "TOP-SECRET") {
			t.Fatalf("%v: output leaks a value: %q", sub, out)
		}
	}
}

func TestLogsAreJSONWithRequestIDAndNoSecrets(t *testing.T) {
	p := start(t, okEnv(), "serve", "--plane=tenant")
	addr := p.waitMsg(t, "listening")["addr"].(string)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/healthz?token=Q-SECRET", nil)
	req.Header.Set("X-Request-Id", "rid-42")
	req.Header.Set("Authorization", "Bearer H-SECRET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	line := p.waitMsg(t, "request")
	if line["request_id"] != "rid-42" {
		t.Fatalf("access log %v", line)
	}
	p.terminate(t)
	if strings.Contains(p.stderr.String(), "SECRET") {
		t.Fatalf("stderr leaks: %s", p.stderr)
	}
}
