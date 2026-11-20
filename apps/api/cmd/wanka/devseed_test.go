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
		{"remote ip", "local", "postgres://postgres:x@10.0.0.5/wanka", false},
	}
	for _, c := range cases {
		if err := checkLocal(env(c.wankaEnv), c.url); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
	}
}
