package db_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bacsystem/wanka/apps/api/internal/platform/db"
)

const secretPassword = "TOP-SECRET-PW"

// closedPortURL devuelve una URL cuyo puerto no tiene nadie escuchando: una base caída.
func closedPortURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return "postgres://wanka_app:" + secretPassword + "@" + addr + "/wanka?sslmode=disable"
}

func TestOpenRejectsBadInputWithoutLeakingTheURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		max  int32
	}{
		{"unparsable port", "postgres://u:" + secretPassword + "@localhost:notaport/x", 1},
		{"unknown scheme", "mysql://u:" + secretPassword + "@localhost/x", 1},
		{"zero pool", "postgres://u:" + secretPassword + "@localhost/x", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Open(context.Background(), tc.url, db.Options{MaxConns: tc.max})
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), secretPassword) || strings.Contains(err.Error(), tc.url) {
				t.Fatalf("error leaks the URL: %v", err)
			}
		})
	}
}

func TestOpenDoesNotConnect(t *testing.T) {
	// Una base caída no debe impedir que el proceso arranque.
	p, err := db.Open(context.Background(), closedPortURL(t), db.Options{MaxConns: 2})
	if err != nil {
		t.Fatalf("Open must be lazy: %v", err)
	}
	p.Close()
}

func TestDatabaseDownIsUnavailableAndSanitized(t *testing.T) {
	p, err := db.Open(context.Background(), closedPortURL(t), db.Options{MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if err := p.Ping(context.Background()); !errors.Is(err, db.ErrUnavailable) {
		t.Fatalf("Ping = %v, want ErrUnavailable", err)
	}
	conn, err := p.Acquire(context.Background())
	if !errors.Is(err, db.ErrUnavailable) {
		t.Fatalf("Acquire = %v, want ErrUnavailable", err)
	}
	if conn != nil {
		conn.Release()
		t.Fatal("got a connection from a dead database")
	}
	if errors.Is(err, db.ErrOverloaded) {
		t.Fatal("a dead database is not an exhausted pool")
	}
	if strings.Contains(err.Error(), secretPassword) {
		t.Fatalf("error leaks the password: %v", err)
	}
}

func TestAcquireHonoursCallerCancellation(t *testing.T) {
	p, err := db.Open(context.Background(), closedPortURL(t), db.Options{MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire = %v, want context.Canceled", err)
	}
}

func TestErrorsStayInspectable(t *testing.T) {
	p, _ := db.Open(context.Background(), closedPortURL(t), db.Options{MaxConns: 1, AcquireTimeout: 50 * time.Millisecond})
	defer p.Close()
	err := p.Ping(context.Background())
	var netErr net.Error
	var opErr *net.OpError
	if !errors.As(err, &opErr) && !errors.As(err, &netErr) {
		t.Fatalf("the cause should stay reachable with errors.As: %v", err)
	}
}

// hungDatabaseURL devuelve la URL de un servidor que acepta conexiones TCP y
// nunca responde: una base colgada (red cortada a medias, proceso congelado).
func hungDatabaseURL(t *testing.T, password string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	})
	return "postgres://wanka_app:" + password + "@" + ln.Addr().String() + "/wanka?sslmode=disable"
}

// Una base colgada no es un pool lleno: el plazo de 200 ms vence igual, pero no
// hay conexiones en uso, así que el error es "no disponible" y no "sobrecarga".
func TestHungDatabaseIsUnavailableNotOverloaded(t *testing.T) {
	p := open(t, hungDatabaseURL(t, "pw"), db.Options{MaxConns: 2})
	start := time.Now()
	conn, err := p.Acquire(context.Background())
	if conn != nil {
		conn.Release()
		t.Fatal("got a connection from a hung database")
	}
	if !errors.Is(err, db.ErrUnavailable) || errors.Is(err, db.ErrOverloaded) {
		t.Fatalf("Acquire = %v, want ErrUnavailable and not ErrOverloaded", err)
	}
	if d := time.Since(start); d < db.DefaultAcquireTimeout || d > 2*time.Second {
		t.Fatalf("answered after %v, want about %v", d, db.DefaultAcquireTimeout)
	}
}

// Con una contraseña de un solo carácter el texto del error no se mutila: la
// contraseña se redacta solo dentro de la URL o de password=.
func TestShortPasswordDoesNotMangleErrors(t *testing.T) {
	p := open(t, hungDatabaseURL(t, "x"), db.Options{MaxConns: 1})
	_, err := p.Acquire(context.Background())
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("error text was mangled: %v", err)
	}
}
