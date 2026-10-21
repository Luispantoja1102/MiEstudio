package api

import (
	"testing"
	"time"
)

func tijuana(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Tijuana")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func clock(t *testing.T, loc *time.Location, y int, mo time.Month, d, h, m int) time.Time {
	return time.Date(y, mo, d, h, m, 0, 0, loc)
}

func TestFreeSlotsBasic(t *testing.T) {
	loc := tijuana(t)
	day := clock(t, loc, 2026, time.November, 9, 0, 0) // lunes
	hours := []clockRange{{9, 0, 14, 0}}
	now := clock(t, loc, 2026, time.November, 1, 0, 0)

	got := freeSlots(loc, day, hours, 30*time.Minute, 0, nil, now)
	// 09:00 … 13:30 cada 15 min = 19 inicios
	if len(got) != 19 || got[0].Hour() != 9 || got[len(got)-1].Hour() != 13 || got[len(got)-1].Minute() != 30 {
		t.Fatalf("slots: %d, primero %v, último %v", len(got), got[0], got[len(got)-1])
	}
}

func TestFreeSlotsBufferAndBusy(t *testing.T) {
	loc := tijuana(t)
	day := clock(t, loc, 2026, time.November, 9, 0, 0)
	hours := []clockRange{{9, 0, 14, 0}}
	now := clock(t, loc, 2026, time.November, 1, 0, 0)
	// cita 09:00–09:30 con 10 min de preparación -> ocupado hasta 09:40
	busy := []interval{{clock(t, loc, 2026, time.November, 9, 9, 0), clock(t, loc, 2026, time.November, 9, 9, 40)}}

	got := freeSlots(loc, day, hours, 30*time.Minute, 10*time.Minute, busy, now)
	if got[0].Hour() != 9 || got[0].Minute() != 45 {
		t.Fatalf("el primer hueco tras la cita debe ser 09:45, fue %v", got[0])
	}
	// un servicio de 30 min que empieza a las 08:45 no existe (fuera de horario); y uno a las 09:15 chocaría
	for _, s := range got {
		if s.Hour() == 9 && s.Minute() < 45 {
			t.Fatalf("hueco inválido %v", s)
		}
	}
}

func TestFreeSlotsBufferMayExceedClosing(t *testing.T) {
	loc := tijuana(t)
	day := clock(t, loc, 2026, time.November, 9, 0, 0)
	now := clock(t, loc, 2026, time.November, 1, 0, 0)
	got := freeSlots(loc, day, []clockRange{{9, 0, 10, 0}}, 30*time.Minute, 15*time.Minute, nil, now)
	// último inicio 09:30 (termina 10:00); la preparación pasa del cierre pero no importa
	if len(got) != 3 || got[2].Minute() != 30 {
		t.Fatalf("%v", got)
	}
}

func TestFreeSlotsSkipsPastAndMultipleBlocks(t *testing.T) {
	loc := tijuana(t)
	day := clock(t, loc, 2026, time.November, 9, 0, 0)
	hours := []clockRange{{9, 0, 11, 0}, {16, 0, 17, 0}}
	now := clock(t, loc, 2026, time.November, 9, 10, 0) // ya son las 10:00

	got := freeSlots(loc, day, hours, 30*time.Minute, 0, nil, now)
	// primer tramo: 10:00, 10:15, 10:30 ; segundo: 16:00, 16:15, 16:30
	if len(got) != 6 || got[0].Hour() != 10 || got[3].Hour() != 16 {
		t.Fatalf("%v", got)
	}
}

func TestFreeSlotsAcrossDSTChange(t *testing.T) {
	loc := tijuana(t)
	// 1 nov 2026: a las 02:00 el reloj vuelve a la 01:00 -> la madrugada dura 7 horas reales
	day := clock(t, loc, 2026, time.November, 1, 0, 0)
	now := clock(t, loc, 2026, time.October, 1, 0, 0)
	got := freeSlots(loc, day, []clockRange{{0, 0, 6, 0}}, 30*time.Minute, 0, nil, now)
	// (7 h − 30 min) / 15 min + 1 = 27 inicios (23 en un día normal)
	if len(got) != 27 {
		t.Fatalf("día de cambio de horario: esperaba 27 inicios, hay %d", len(got))
	}
	normal := freeSlots(loc, clock(t, loc, 2026, time.November, 2, 0, 0), []clockRange{{0, 0, 6, 0}}, 30*time.Minute, 0, nil, now)
	if len(normal) != 23 {
		t.Fatalf("día normal: esperaba 23, hay %d", len(normal))
	}
}
