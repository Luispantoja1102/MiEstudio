package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New()
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("k", 3, time.Minute) {
			t.Fatalf("intento %d debería pasar", i)
		}
	}
	if l.Allow("k", 3, time.Minute) {
		t.Fatal("el cuarto intento debería bloquearse")
	}
	if !l.Allow("otra", 3, time.Minute) {
		t.Fatal("otra clave no debe verse afectada")
	}
	now = now.Add(61 * time.Second)
	if !l.Allow("k", 3, time.Minute) {
		t.Fatal("tras la ventana debe volver a permitir")
	}
}
