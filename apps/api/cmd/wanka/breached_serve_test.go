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

// El cableado de producción usa el archivo embebido, que carga.
func TestProductionUsesEmbeddedBreachedList(t *testing.T) {
	if defaultWiring.breachedList != nil {
		t.Fatal("defaultWiring must not override the embedded breached list")
	}
	p, err := loadPasswordPolicy(defaultWiring.breachedList)
	if err != nil || p.Len() == 0 {
		t.Fatalf("embedded list: %v", err)
	}
}
