package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Recordatorios por WhatsApp (API en la nube de Meta). Salen por la misma bandeja que los correos, con su canal propio:
// reintentos, comprobación de que la cita siga vigente y clave de duplicado. Solo van a quien aceptó WhatsApp
// (clients.whatsapp_consent) y solo si el negocio los encendió. La persona se da de baja respondiendo «BAJA» (webhook).

func (a *API) waEnabled() bool { return a.WhatsApp != nil && a.WhatsApp.Enabled() }

var nonDigits = regexp.MustCompile(`\D`)

// waNumber da el número en el formato de la API: lada de país y 10 dígitos, sin signos (México: 52…).
func waNumber(phone string) string {
	d := nonDigits.ReplaceAllString(phone, "")
	if len(d) < 10 {
		return ""
	}
	return "52" + d[len(d)-10:]
}

// samePhone compara un teléfono guardado con el número al que se envió, ignorando la lada.
func samePhone(stored, sentTo string) bool {
	a, b := nonDigits.ReplaceAllString(stored, ""), nonDigits.ReplaceAllString(sentTo, "")
	return len(a) >= 10 && len(b) >= 10 && a[len(a)-10:] == b[len(b)-10:]
}

type waPayload struct {
	Params []string `json:"params"`
}

// enqueueWhatsAppReminders crea los recordatorios por WhatsApp que ya tocan: mismas reglas de tiempo que el correo,
// pero con el consentimiento de WhatsApp y el interruptor del negocio.
func (a *API) enqueueWhatsAppReminders(ctx context.Context) error {
	now := a.now()
	rows, err := a.DB.Query(ctx,
		`SELECT ap.id FROM appointments ap
		   JOIN clients c ON c.id = ap.client_id
		   JOIN businesses b ON b.id = ap.business_id
		   JOIN booking_settings bs ON bs.business_id = ap.business_id
		  WHERE ap.status = 'scheduled' AND `+usableSQL("b.", "$1")+` AND bs.reminder_hours > 0 AND bs.whatsapp_reminders
		    AND c.whatsapp_consent AND c.anonymized_at IS NULL AND length(regexp_replace(c.phone, '\D', '', 'g')) >= 10
		    AND ap.starts_at > $1 AND ap.starts_at <= $1 + bs.reminder_hours * interval '1 hour'
		    AND ap.created_at <= ap.starts_at - bs.reminder_hours * interval '1 hour'
		    AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.appointment_id = ap.id AND n.kind = 'reminder' AND n.for_starts_at = ap.starts_at AND n.channel = 'whatsapp')
		  ORDER BY ap.starts_at LIMIT 200`, now)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := a.enqueueWhatsApp(ctx, a.DB, id); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) enqueueWhatsApp(ctx context.Context, q execQueryer, apptID int64) error {
	m, err := loadMailCtx(ctx, q, apptID)
	if err != nil {
		return err
	}
	var phone string
	if err := q.QueryRow(ctx, `SELECT c.phone FROM appointments ap JOIN clients c ON c.id = ap.client_id WHERE ap.id = $1`, apptID).Scan(&phone); err != nil {
		return err
	}
	to := waNumber(phone)
	if to == "" {
		return nil
	}
	payload, _ := json.Marshal(waPayload{Params: []string{firstName(m.ClientName), m.Service, whenES(m.Starts, m.Loc), m.Business}})
	_, err = q.Exec(ctx,
		`INSERT INTO notifications (business_id, appointment_id, kind, for_starts_at, to_addr, subject, body_text, send_after, channel, payload)
		 VALUES ($1, $2, 'reminder', $3, $4, '', '', $5, 'whatsapp', $6) ON CONFLICT (appointment_id, kind, for_starts_at, channel) DO NOTHING`,
		m.BizID, apptID, m.Starts, to, a.now(), payload)
	return err
}

func (a *API) sendWhatsApp(ctx context.Context, q queued) error {
	var p waPayload
	if err := json.Unmarshal([]byte(q.Payload), &p); err != nil {
		return err
	}
	tpl, lang := a.WhatsApp.Reminder()
	return a.WhatsApp.SendTemplate(ctx, q.To, tpl, lang, p.Params)
}

// ---------- webhook de Meta: verificación y bajas ----------

// whatsappVerify responde al reto con el que Meta comprueba la dirección del webhook.
func (a *API) whatsappVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if a.WAVerifyToken == "" || q.Get("hub.mode") != "subscribe" ||
		subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(a.WAVerifyToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(q.Get("hub.challenge")))
}

func validWASignature(secret string, body []byte, header string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(header))
}

// optOutWords: lo que escribe quien ya no quiere recibir mensajes.
var optOutWords = map[string]bool{"baja": true, "stop": true, "detener": true, "parar": true, "cancelar": true, "no mas": true, "no quiero": true, "unsubscribe": true}

func isOptOut(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "¡", "", "!", "", ".", "").Replace(t)
	if optOutWords[t] {
		return true
	}
	first, _, _ := strings.Cut(t, " ")
	return first == "baja" || first == "stop"
}

type waInbound struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []struct {
					From string `json:"from"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
					Button struct {
						Text string `json:"text"`
					} `json:"button"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// whatsappWebhook recibe los mensajes de las personas; quien responde «BAJA» deja de recibir recordatorios por WhatsApp.
// Sin WHATSAPP_APP_SECRET el webhook no existe: no se acepta un aviso que no se pueda verificar.
func (a *API) whatsappWebhook(w http.ResponseWriter, r *http.Request) {
	if a.WAAppSecret == "" {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !validWASignature(a.WAAppSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var in waInbound
	if err := json.Unmarshal(body, &in); err != nil {
		w.WriteHeader(http.StatusOK) // no es algo que entendamos; Meta no debe reintentarlo
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	for _, e := range in.Entry {
		for _, c := range e.Changes {
			for _, m := range c.Value.Messages {
				text := m.Text.Body
				if m.Type == "button" {
					text = m.Button.Text
				}
				if !isOptOut(text) || len(nonDigits.ReplaceAllString(m.From, "")) < 10 {
					continue
				}
				if _, err := a.DB.Exec(ctx,
					`UPDATE clients SET whatsapp_consent = false WHERE whatsapp_consent
					   AND right(regexp_replace(phone, '\D', '', 'g'), 10) = right(regexp_replace($1, '\D', '', 'g'), 10)`, m.From); err != nil {
					serverErr(w)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}
