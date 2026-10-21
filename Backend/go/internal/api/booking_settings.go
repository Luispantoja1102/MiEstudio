package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

type bookingSettings struct {
	Enabled        bool   `json:"enabled"`
	MinNoticeHours int    `json:"min_notice_hours"`
	MaxDaysAhead   int    `json:"max_days_ahead"`
	CancelMinHours int    `json:"cancel_min_hours"`
	DepositPercent int    `json:"deposit_percent"`
	ReminderHours  int    `json:"reminder_hours"` // 0 = sin recordatorios
	ShowPrices     bool   `json:"show_prices"`
	WelcomeText    string `json:"welcome_text"`
}

func defaultBookingSettings() bookingSettings {
	return bookingSettings{MinNoticeHours: 2, MaxDaysAhead: 30, CancelMinHours: 2, ReminderHours: 24, ShowPrices: true}
}

func loadBookingSettings(ctx context.Context, q queryer, bizID int64) (bookingSettings, error) {
	s := defaultBookingSettings()
	err := q.QueryRow(ctx,
		`SELECT enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text
		 FROM booking_settings WHERE business_id = $1`, bizID).
		Scan(&s.Enabled, &s.MinNoticeHours, &s.MaxDaysAhead, &s.CancelMinHours, &s.DepositPercent, &s.ReminderHours, &s.ShowPrices, &s.WelcomeText)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	return s, err
}

func (a *API) mpConnected(ctx context.Context, bizID int64) bool {
	var ok bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mp_accounts WHERE business_id = $1)`, bizID).Scan(&ok)
	return ok
}

// getBookingSettings devuelve los ajustes de reserva en línea junto con lo necesario para mostrarlos.
func (a *API) getBookingSettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	s, err := loadBookingSettings(ctx, a.DB, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	var slug string
	if err := a.DB.QueryRow(ctx, `SELECT slug FROM businesses WHERE id = $1`, p.BusinessID).Scan(&slug); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": s, "slug": slug, "public_path": "/reservar/" + slug,
		"mp_connected": a.mpConnected(ctx, p.BusinessID), "mail_enabled": a.mailEnabled(),
	})
}

func validBookingSettings(s bookingSettings) string {
	switch {
	case s.MinNoticeHours < 0 || s.MinNoticeHours > 720:
		return "el aviso mínimo debe estar entre 0 y 720 horas"
	case s.MaxDaysAhead < 1 || s.MaxDaysAhead > 365:
		return "los días de anticipación deben estar entre 1 y 365"
	case s.CancelMinHours < 0 || s.CancelMinHours > 720:
		return "el tiempo mínimo para cancelar debe estar entre 0 y 720 horas"
	case s.DepositPercent < 0 || s.DepositPercent > 100:
		return "el anticipo debe estar entre 0 % y 100 %"
	case s.ReminderHours < 0 || s.ReminderHours > 168:
		return "el recordatorio debe ser de 0 a 168 horas antes (0 = sin recordatorio)"
	case utf8.RuneCountInString(s.WelcomeText) > 500:
		return "el mensaje de bienvenida es demasiado largo (máximo 500 caracteres)"
	}
	return ""
}

func (a *API) putBookingSettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var s bookingSettings
	if !decode(w, r, &s) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	s.WelcomeText = strings.TrimSpace(s.WelcomeText)
	if msg := validBookingSettings(s); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	// un anticipo solo se puede cobrar si hay una cuenta de Mercado Pago conectada
	if s.DepositPercent > 0 && !a.mpConnected(r.Context(), p.BusinessID) {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "para cobrar anticipo conecta primero tu cuenta de Mercado Pago (Pagos)", "code": "mp_required"})
		return
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO booking_settings (business_id, enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (business_id) DO UPDATE SET enabled = EXCLUDED.enabled, min_notice_hours = EXCLUDED.min_notice_hours,
		   max_days_ahead = EXCLUDED.max_days_ahead, cancel_min_hours = EXCLUDED.cancel_min_hours, deposit_percent = EXCLUDED.deposit_percent,
		   reminder_hours = EXCLUDED.reminder_hours, show_prices = EXCLUDED.show_prices, welcome_text = EXCLUDED.welcome_text`,
		p.BusinessID, s.Enabled, s.MinNoticeHours, s.MaxDaysAhead, s.CancelMinHours, s.DepositPercent, s.ReminderHours, s.ShowPrices, s.WelcomeText); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "booking_settings_changed", map[string]any{"enabled": s.Enabled, "deposit_percent": s.DepositPercent})
	w.WriteHeader(http.StatusNoContent)
}
