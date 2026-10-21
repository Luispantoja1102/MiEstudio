package api

import (
	"time"
)

const slotStep = 15 * time.Minute

type interval struct{ start, end time.Time }

// clockRange es un tramo de horario dentro de un día (hora local del negocio).
type clockRange struct{ startH, startM, endH, endM int }

func overlaps(busy []interval, start, end time.Time) bool {
	for _, b := range busy {
		if start.Before(b.end) && b.start.Before(end) {
			return true
		}
	}
	return false
}

// freeSlots devuelve las horas de inicio disponibles para un servicio en un día.
//
//   - hours: tramos de trabajo del profesional ese día, en hora local.
//   - dur:   duración del servicio; debe caber completa dentro del tramo.
//   - buf:   preparación posterior; se aparta pero puede exceder el cierre.
//   - busy:  citas (ya con su preparación) y bloqueos del profesional.
//
// Las horas se arman con time.Date y se avanza en tiempo absoluto, así los días
// con cambio de horario de verano producen los huecos reales.
func freeSlots(loc *time.Location, day time.Time, hours []clockRange, dur, buf time.Duration, busy []interval, now time.Time) []time.Time {
	y, m, d := day.In(loc).Date()
	var out []time.Time
	for _, h := range hours {
		ws := time.Date(y, m, d, h.startH, h.startM, 0, 0, loc)
		we := time.Date(y, m, d, h.endH, h.endM, 0, 0, loc)
		for t := ws; !t.Add(dur).After(we); t = t.Add(slotStep) {
			if t.Before(now) || overlaps(busy, t, t.Add(dur+buf)) {
				continue
			}
			out = append(out, t)
		}
	}
	return out
}
