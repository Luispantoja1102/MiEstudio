// Package ratelimit limita intentos por clave en una ventana de tiempo (en memoria).
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
	now  func() time.Time
}

func New() *Limiter {
	return &Limiter{hits: map[string][]time.Time{}, now: time.Now}
}

// Allow registra un intento y dice si cabe dentro de max por window.
func (l *Limiter) Allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-window)
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= max {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)

	// evita crecer sin límite con claves que ya no se usan
	if len(l.hits) > 10000 {
		for k, v := range l.hits {
			if len(v) == 0 || !v[len(v)-1].After(now.Add(-time.Hour)) {
				delete(l.hits, k)
			}
		}
	}
	return true
}
