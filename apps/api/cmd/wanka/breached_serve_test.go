package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/config"
	"github.com/bacsystem/wanka/apps/api/internal/platform/passwordpolicy"
)

// serve falla cerrado si la lista de contraseñas filtradas falta o está dañada (#238). Falla
// antes de abrir la base: la configuración vacía no se llega a usar.
func TestServeRefusesBadBreachedList(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, data := range map[string][]byte{
		"empty file":   {},
		"junk":         []byte("not a breached list at all, but longer than the header is........"),
		"bad checksum": []byte("WKBP\x01\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00" + string(make([]byte, 40))),
	} {
		t.Run(name, func(t *testing.T) {
			w := defaultWiring
			w.breachedList = data
			err := serve(context.Background(), log, config.Config{}, config.API, "tenant", w)
			if !errors.Is(err, passwordpolicy.ErrCorrupt) {
				t.Fatalf("serve = %v, want passwordpolicy.ErrCorrupt", err)
			}
		})
	}
}

// El cableado de producción carga la lista embebida y deja la política en wiring: es el
// objeto que serve pasa a mountPlane y del que tomarán las rutas de contraseña.
func TestProductionWiringHoldsPasswordPolicy(t *testing.T) {
	if defaultWiring.breachedList != nil {
		t.Fatal("defaultWiring must not override the embedded breached list")
	}
	if defaultWiring.passwords != nil {
		t.Fatal("defaultWiring must not carry a policy before serve loads it")
	}
	w, err := defaultWiring.withPasswords()
	if err != nil || w.passwords == nil {
		t.Fatalf("withPasswords: %v, policy %v", err, w.passwords)
	}
	if !errors.Is(w.passwords.Check("password123"), passwordpolicy.ErrBreached) || w.passwords.Check("PASSWORD123") != nil {
		t.Fatal("the wired policy is not the embedded list")
	}
	bad := defaultWiring
	bad.breachedList = []byte{}
	if bad, err = bad.withPasswords(); !errors.Is(err, passwordpolicy.ErrCorrupt) || bad.passwords != nil {
		t.Fatalf("a corrupt list must not wire a policy: %v", err)
	}
}
