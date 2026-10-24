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
	Enabled           bool   `json:"enabled"`
	MinNoticeHours    int    `json:"min_notice_hours"`
	MaxDaysAhead      int    `json:"max_days_ahead"`
	CancelMinHours    int    `json:"cancel_min_hours"`
	DepositPercent    int    `json:"deposit_percent"`
	ReminderHours     int    `json:"reminder_hours"` // 0 = sin recordatorios
	ShowPrices        bool   `json:"show_prices"`
	WelcomeText       string `json:"welcome_text"`
	PrivacyEmail      string `json:"privacy_email"`          // correo para solicitudes de privacidad (se muestra en el aviso)
	PublicTheme       string `json:"public_theme,omitempty"` // auto | light | ivory | dark: tema de las páginas públicas del negocio
	WhatsAppReminders bool   `json:"whatsapp_reminders"`     // además del correo, recordar por WhatsApp a quien lo aceptó

	// anticipo por transferencia: datos que se muestran a quien reserva
	TransferEnabled   bool   `json:"transfer_enabled"`
	TransferBank      string `json:"transfer_bank"`
	TransferHolder    string `json:"transfer_holder"`
	TransferClabe     string `json:"transfer_clabe"`      // 18 dígitos; puede ir vacío si hay tarjeta
	TransferCard      string `json:"transfer_card"`       // 15 o 16 dígitos; puede ir vacío si hay CLABE
	TransferWhatsApp  string `json:"transfer_whatsapp"`   // número al que se puede enviar el comprobante
	TransferHoldHours int    `json:"transfer_hold_hours"` // cuánto se aparta el horario mientras llega el comprobante
}

func defaultBookingSettings() bookingSettings {
	return bookingSettings{MinNoticeHours: 2, MaxDaysAhead: 30, CancelMinHours: 2, ReminderHours: 24, ShowPrices: true, PublicTheme: "auto", TransferHoldHours: 12}
}

func loadBookingSettings(ctx context.Context, q queryer, bizID int64) (bookingSettings, error) {
	s := defaultBookingSettings()
	err := q.QueryRow(ctx,
		`SELECT enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text, privacy_email, whatsapp_reminders, public_theme,
		        transfer_enabled, transfer_bank, transfer_holder, transfer_clabe, transfer_card, transfer_whatsapp, transfer_hold_hours
		 FROM booking_settings WHERE business_id = $1`, bizID).
		Scan(&s.Enabled, &s.MinNoticeHours, &s.MaxDaysAhead, &s.CancelMinHours, &s.DepositPercent, &s.ReminderHours, &s.ShowPrices, &s.WelcomeText, &s.PrivacyEmail, &s.WhatsAppReminders, &s.PublicTheme,
			&s.TransferEnabled, &s.TransferBank, &s.TransferHolder, &s.TransferClabe, &s.TransferCard, &s.TransferWhatsApp, &s.TransferHoldHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	return s, err
}

// isOwner dice si la persona es la titular del negocio: quien lo registró (el primer administrador). Si esa persona
// ya no es administradora activa, el título pasa al administrador activo más antiguo, para que nunca quede sin titular.
func (a *API) isOwner(ctx context.Context, bizID, userID int64) bool {
	var ok bool
	a.DB.QueryRow(ctx,
		`SELECT COALESCE(
		   (SELECT u.id FROM businesses b JOIN users u ON u.id = b.owner_user_id AND u.active AND u.role = 'admin' WHERE b.id = $1),
		   (SELECT id FROM users WHERE business_id = $1 AND role = 'admin' AND active ORDER BY id LIMIT 1)) = $2`, bizID, userID).Scan(&ok)
	return ok
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
	// la apariencia de la página pública es del titular: a los demás ni siquiera se les muestra
	owner := a.isOwner(ctx, p.BusinessID, p.UserID)
	if !owner {
		s.PublicTheme = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": s, "slug": slug, "public_path": "/reservar/" + slug, "is_owner": owner,
		"mp_connected": a.mpConnected(ctx, p.BusinessID), "plan_features": a.planFeatures(ctx, p.BusinessID), "mail_enabled": a.mailEnabled(), "whatsapp_available": a.waEnabled(),
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
	case s.PrivacyEmail != "" && !validEmail(s.PrivacyEmail):
		return "el correo de privacidad no es válido"
	case !validPublicTheme(s.PublicTheme):
		return "el tema de la página pública no es válido"
	case s.TransferEnabled && (strings.TrimSpace(s.TransferBank) == "" || strings.TrimSpace(s.TransferHolder) == ""):
		return "para pedir transferencias escribe el banco y el titular de la cuenta"
	case s.TransferEnabled && s.TransferClabe == "" && s.TransferCard == "":
		return "captura tu CLABE, el número de tu tarjeta o los dos para recibir transferencias"
	case s.TransferClabe != "" && !validClabe(s.TransferClabe):
		return "la CLABE no es válida: deben ser 18 dígitos y el último es un dígito verificador; revísala"
	case s.TransferCard != "" && !validCard(s.TransferCard):
		return "el número de tarjeta no es válido: deben ser 15 o 16 dígitos; revísalo"
	case utf8.RuneCountInString(s.TransferBank) > 60 || utf8.RuneCountInString(s.TransferHolder) > 100:
		return "el banco o el titular son demasiado largos"
	case s.TransferWhatsApp != "" && normalizePhone(s.TransferWhatsApp) == "":
		return "el WhatsApp para comprobantes no es válido"
	case s.TransferHoldHours < 1 || s.TransferHoldHours > 168:
		return "el tiempo para enviar el comprobante debe ser de 1 a 168 horas"
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
	s.PrivacyEmail = lowerTrim(s.PrivacyEmail)
	s.TransferBank, s.TransferHolder = strings.TrimSpace(s.TransferBank), strings.TrimSpace(s.TransferHolder)
	s.TransferClabe, s.TransferCard = onlyDigits(s.TransferClabe), onlyDigits(s.TransferCard)
	s.TransferWhatsApp = strings.TrimSpace(s.TransferWhatsApp)
	if s.TransferHoldHours == 0 {
		s.TransferHoldHours = 12 // clientes que no envían el campo
	}
	// sin el campo (otros administradores, clientes antiguos) el tema no cambia; solo el titular puede cambiarlo
	cur, err := loadBookingSettings(r.Context(), a.DB, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	if s.PublicTheme == "" {
		s.PublicTheme = cur.PublicTheme
	} else if s.PublicTheme != cur.PublicTheme && !a.isOwner(r.Context(), p.BusinessID, p.UserID) {
		writeErr(w, http.StatusForbidden, "solo el titular del negocio puede cambiar el tema de la página pública")
		return
	}
	if msg := validBookingSettings(s); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if s.WhatsAppReminders && !a.waEnabled() {
		writeErr(w, http.StatusConflict, "WhatsApp no está configurado en este servidor todavía")
		return
	}
	if s.DepositPercent > 0 {
		pl := a.planOf(r.Context(), a.DB, p.BusinessID)
		if !pl.Deposits && !pl.DepositsMP {
			writeFail(w, errPlanDeposits)
			return
		}
		// un anticipo solo se puede cobrar por un método que el plan permita y que esté listo:
		// transferencias activadas (todos los planes con anticipo) o Mercado Pago conectado (solo si el plan lo incluye)
		canTransfer := pl.Deposits && s.TransferEnabled
		canMP := pl.DepositsMP && a.mpConnected(r.Context(), p.BusinessID)
		if !canTransfer && !canMP {
			if !pl.DepositsMP {
				writeJSON(w, http.StatusConflict, map[string]string{
					"error": "tu plan cobra anticipos por transferencia: activa las transferencias y captura tu CLABE", "code": "transfer_required"})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "para cobrar anticipo conecta primero tu cuenta de Mercado Pago (Pagos) o activa las transferencias", "code": "mp_required"})
			return
		}
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO booking_settings (business_id, enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text, privacy_email, whatsapp_reminders, public_theme,
		   transfer_enabled, transfer_bank, transfer_holder, transfer_clabe, transfer_card, transfer_whatsapp, transfer_hold_hours)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		 ON CONFLICT (business_id) DO UPDATE SET enabled = EXCLUDED.enabled, min_notice_hours = EXCLUDED.min_notice_hours,
		   max_days_ahead = EXCLUDED.max_days_ahead, cancel_min_hours = EXCLUDED.cancel_min_hours, deposit_percent = EXCLUDED.deposit_percent,
		   reminder_hours = EXCLUDED.reminder_hours, show_prices = EXCLUDED.show_prices, welcome_text = EXCLUDED.welcome_text, privacy_email = EXCLUDED.privacy_email,
		   whatsapp_reminders = EXCLUDED.whatsapp_reminders, public_theme = EXCLUDED.public_theme,
		   transfer_enabled = EXCLUDED.transfer_enabled, transfer_bank = EXCLUDED.transfer_bank, transfer_holder = EXCLUDED.transfer_holder,
		   transfer_clabe = EXCLUDED.transfer_clabe, transfer_card = EXCLUDED.transfer_card, transfer_whatsapp = EXCLUDED.transfer_whatsapp, transfer_hold_hours = EXCLUDED.transfer_hold_hours`,
		p.BusinessID, s.Enabled, s.MinNoticeHours, s.MaxDaysAhead, s.CancelMinHours, s.DepositPercent, s.ReminderHours, s.ShowPrices, s.WelcomeText, s.PrivacyEmail, s.WhatsAppReminders, s.PublicTheme,
		s.TransferEnabled, s.TransferBank, s.TransferHolder, s.TransferClabe, s.TransferCard, s.TransferWhatsApp, s.TransferHoldHours); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "booking_settings_changed", map[string]any{"enabled": s.Enabled, "deposit_percent": s.DepositPercent})
	w.WriteHeader(http.StatusNoContent)
}

func validPublicTheme(t string) bool {
	switch t {
	case "auto", "lino", "light", "ivory", "dark":
		return true
	}
	return false
}

// validClabe: 18 dígitos (no se valida el dígito de control: lo que se muestra al cliente lo escribe el titular).
func onlyDigits(v string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, v)
}

// validClabe: 18 dígitos cuyo último es el verificador (pesos 3, 7, 1 sobre los 17 primeros).
func validClabe(c string) bool {
	if len(c) != 18 || onlyDigits(c) != c {
		return false
	}
	w := [3]int{3, 7, 1}
	sum := 0
	for i := 0; i < 17; i++ {
		sum += (int(c[i]-'0') * w[i%3]) % 10
	}
	return int(c[17]-'0') == (10-sum%10)%10
}

// validCard: 15 o 16 dígitos que pasan la verificación de Luhn (la de todas las tarjetas bancarias).
func validCard(c string) bool {
	if (len(c) != 15 && len(c) != 16) || onlyDigits(c) != c {
		return false
	}
	sum := 0
	for i := 0; i < len(c); i++ {
		d := int(c[len(c)-1-i] - '0')
		if i%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// waDigits deja el número listo para wa.me: con lada de país (52 si solo trae los 10 dígitos de México).
func waDigits(raw string) string {
	d := normalizePhone(raw)
	if len(d) == 10 {
		return "52" + d
	}
	return d
}
