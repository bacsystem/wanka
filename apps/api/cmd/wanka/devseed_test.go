package main

import "testing"

func TestCheckLocal(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	cases := []struct {
		name, wankaEnv, url string
		ok                  bool
	}{
		{"local loopback", "local", "postgres://postgres:x@127.0.0.1:55432/wanka", true},
		{"local localhost", "local", "postgres://postgres:x@localhost/wanka", true},
		{"no env", "", "postgres://postgres:x@127.0.0.1/wanka", false},
		{"other env", "production", "postgres://postgres:x@127.0.0.1/wanka", false},
		{"remote host", "local", "postgres://postgres:x@db.example.com/wanka", false},
		{"ipv6 loopback", "local", "postgres://postgres:x@[::1]:5432/wanka", true},
		{"unix socket", "local", "postgres://postgres:x@/wanka?host=/var/run/postgresql", true},
		{"host param remote", "local", "postgres://postgres:x@127.0.0.1/wanka?host=10.255.255.1", false},
		{"host param name", "local", "postgres://postgres:x@127.0.0.1/wanka?host=db.example.com", false},
		{"comma list with remote", "local", "postgres://postgres:x@127.0.0.1,10.0.0.5/wanka", false},
		{"fallback remote", "local", "postgres://postgres:x@127.0.0.1/wanka?host=127.0.0.1,db.example.com", false},
		{"service file", "local", "postgres://postgres:x@127.0.0.1/wanka?service=prod", false},
		{"passfile", "local", "postgres://postgres:x@127.0.0.1/wanka?passfile=/tmp/x", false},
		{"hostaddr", "local", "postgres://postgres:x@127.0.0.1/wanka?hostaddr=10.0.0.5", false},
		{"no host defaults to local", "local", "postgres:///wanka", true},
		{"remote ip", "local", "postgres://postgres:x@10.0.0.5/wanka", false},
	}
	for _, c := range cases {
		if err := checkLocal(env(c.wankaEnv), c.url); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
	}
}
