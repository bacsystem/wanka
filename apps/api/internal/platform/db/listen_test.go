package db_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
	"github.com/bacsystem/wanka/apps/api/internal/platform/db/dbtest"
)

// listening corre Listen en una goroutine que el test termina y espera.
type listening struct {
	got       chan string
	connected atomic.Int64
	errs      atomic.Int64
	stop      func()
}

func startListen(t *testing.T, p *db.Pool, channel string) *listening {
	t.Helper()
	l := &listening{got: make(chan string, 16)}
	c, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = p.Listen(c, channel, func(s string) { l.got <- s }, db.ListenHooks{
			OnConnect: func() { l.connected.Add(1) },
			OnError:   func(error) { l.errs.Add(1) },
		})
	}()
	l.stop = func() { cancel(); wg.Wait() }
	t.Cleanup(l.stop)
	return l
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func openApp(t *testing.T, d *dbtest.DB) *db.Pool {
	t.Helper()
	p, err := db.Open(context.Background(), d.URL(dbtest.App), db.Options{MaxConns: 2, AppName: "listen-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestListenDeliversNotificationsAndDoesNotUseThePool(t *testing.T) {
	d := dbtest.New(t)
	p := openApp(t, d)
	admin := d.Admin(context.Background(), t)
	l := startListen(t, p, "some_channel")
	eventually(t, "LISTEN active", func() bool { return l.connected.Load() == 1 })

	if _, err := admin.Exec(context.Background(), `SELECT pg_notify('some_channel', 'hola')`); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-l.got:
		if got != "hola" {
			t.Errorf("payload = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
	}
	if n := p.AcquiredConns(); n != 0 {
		t.Errorf("the listener holds %d pool connections, want 0 (own connection)", n)
	}
}

// Si la conexión se pierde, Listen reconecta, avisa con OnConnect (hay que descartar lo cacheado) y sigue entregando.
func TestListenReconnectsAfterTheConnectionIsKilled(t *testing.T) {
	d := dbtest.New(t)
	p := openApp(t, d)
	admin := d.Admin(context.Background(), t)
	l := startListen(t, p, "some_channel")
	eventually(t, "LISTEN active", func() bool { return l.connected.Load() == 1 })

	if _, err := admin.Exec(context.Background(),
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name = 'listen-test-listen'`); err != nil {
		t.Fatal(err)
	}
	eventually(t, "reconnected", func() bool { return l.connected.Load() == 2 })
	if l.errs.Load() == 0 {
		t.Error("OnError was not called for the lost connection")
	}
	if _, err := admin.Exec(context.Background(), `SELECT pg_notify('some_channel', 'despues')`); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-l.got:
		if got != "despues" {
			t.Errorf("payload = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notification after reconnecting")
	}
}

// Cancelar el contexto termina Listen y cierra su conexión.
func TestListenStopsWithTheContext(t *testing.T) {
	d := dbtest.New(t)
	p := openApp(t, d)
	admin := d.Admin(context.Background(), t)
	l := startListen(t, p, "some_channel")
	eventually(t, "LISTEN active", func() bool { return l.connected.Load() == 1 })
	l.stop()
	eventually(t, "listener connection closed", func() bool {
		var n int
		_ = admin.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE application_name = 'listen-test-listen'`).Scan(&n)
		return n == 0
	})
}

// Con la base caída reintenta sin tope y termina limpio al cancelar; el error no lleva la clave.
func TestListenRetriesWhileTheDatabaseIsDownAndNeverLeaksThePassword(t *testing.T) {
	p, err := db.Open(context.Background(), "postgres://wanka_app:s3cr3t-clave@127.0.0.1:1/wanka", db.Options{MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	var mu sync.Mutex
	var errs []error
	c, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = p.Listen(c, "x", func(string) {}, db.ListenHooks{OnError: func(e error) { mu.Lock(); errs = append(errs, e); mu.Unlock() }})
	}()
	eventually(t, "two failed attempts", func() bool { mu.Lock(); defer mu.Unlock(); return len(errs) >= 2 })
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Listen did not stop after the context was canceled")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, e := range errs {
		if s := e.Error(); strings.Contains(s, "s3cr3t-clave") {
			t.Errorf("error leaks the password: %v", e)
		}
	}
}
