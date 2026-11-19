package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

const secretURL = "postgres://u:TOP-SECRET-PW@127.0.0.1:1/x?sslmode=disable" // puerto 1: nadie escucha, la base está "caída"

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

// allURLs fija la variable de URL de cada servicio.
func allURLs(url string) []string {
	return []string{
		"DATABASE_URL_APP=" + url, "DATABASE_URL_PLATFORM=" + url, "DATABASE_URL_WORKER=" + url,
		"DATABASE_URL_WEBHOOKS=" + url, "DATABASE_URL_MIGRATOR=" + url,
	}
}

func okEnv() []string { return append(allURLs(secretURL), "HTTP_ADDR=127.0.0.1:0") }

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
			// La base está caída: no está lista, pero el proceso vive.
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

// Sin base el worker no puede levantar sus colas: sale con código 1 y sin la contraseña.
func TestWorkerExitsWithErrorWhenTheDatabaseIsDown(t *testing.T) {
	cmd := exec.Command(binPath, "worker")
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, okEnv()...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("want exit code 1, got %v; output: %s", err, out)
	}
	if !strings.Contains(string(out), "starting the job queues") || strings.Contains(string(out), "TOP-SECRET") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestWorkerRejectsABadQueueSetting(t *testing.T) {
	cmd := exec.Command(binPath, "worker")
	cmd.Env = append(append([]string{"PATH=" + os.Getenv("PATH")}, okEnv()...), "JOBS_CONCURRENCY_CPE=lots-123")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("want exit code 1, got %v; output: %s", err, out)
	}
	if !strings.Contains(string(out), "JOBS_CONCURRENCY_CPE") || strings.Contains(string(out), "lots-123") {
		t.Fatalf("the error must name the variable and not its value: %s", out)
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
		{"migrate without action", []string{"migrate"}, "up"},
		{"migrate unknown action", []string{"migrate", "down"}, "up"},
		{"migrate extra args", []string{"migrate", "up", "now"}, "up"},
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
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"serve", "--plane=tenant"}, "DATABASE_URL_APP"},
		{[]string{"serve", "--plane=platform"}, "DATABASE_URL_PLATFORM"},
		{[]string{"serve", "--plane=webhooks"}, "DATABASE_URL_WEBHOOKS"},
		{[]string{"worker"}, "DATABASE_URL_WORKER"},
		{[]string{"migrate", "up"}, "DATABASE_URL_MIGRATOR"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := exec.Command(binPath, tc.args...)
			// Los demás servicios tienen su URL; falta la que este comando necesita.
			var env []string
			for _, kv := range allURLs(secretURL) {
				if !strings.HasPrefix(kv, tc.want+"=") {
					env = append(env, kv)
				}
			}
			cmd.Env = append(env, "HTTP_ADDR=127.0.0.1:0", "LOG_LEVEL="+secretURL)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("expected failure")
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("output %q does not name %s", out, tc.want)
			}
			if strings.Contains(string(out), "TOP-SECRET") {
				t.Fatalf("output leaks a value: %q", out)
			}
		})
	}
}

func TestDatabaseDownKeepsTheProcessAliveAndReadyzSaysSo(t *testing.T) {
	p := start(t, okEnv(), "serve", "--plane=tenant")
	addr := p.waitMsg(t, "listening")["addr"].(string)

	for range 3 {
		resp, err := http.Get("http://" + addr + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		var problem map[string]any
		err = json.NewDecoder(resp.Body).Decode(&problem)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 503 || problem["code"] != "dependency_unavailable" {
			t.Fatalf("readyz = %d %v (%v)", resp.StatusCode, problem, err)
		}
		if resp.Header.Get("Retry-After") != "1" {
			t.Fatalf("Retry-After = %q", resp.Header.Get("Retry-After"))
		}
		resp, err = http.Get("http://" + addr + "/healthz")
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("healthz after a failed readyz: %v %v", resp, err)
		}
		resp.Body.Close()
	}
	// El fallo se registra, sin el secreto.
	line := p.waitMsg(t, "readiness check failed")
	if strings.Contains(fmt.Sprint(line), "TOP-SECRET") {
		t.Fatalf("log leaks the password: %v", line)
	}
	p.terminate(t)
	if strings.Contains(p.stderr.String(), "TOP-SECRET") {
		t.Fatalf("stderr leaks: %s", p.stderr)
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
