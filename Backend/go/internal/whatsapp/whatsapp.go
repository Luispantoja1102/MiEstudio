// Package whatsapp envía mensajes de plantilla por la API en la nube de WhatsApp Business (Meta).
//
// Fuera de la ventana de 24 horas, WhatsApp solo permite iniciar la conversación con una plantilla que Meta ya aprobó;
// por eso aquí solo existe SendTemplate. La plantilla se crea y se aprueba en el administrador de WhatsApp antes de usarla.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Sender entrega mensajes de plantilla. La API depende de esta interfaz para poder capturarlos en las pruebas.
type Sender interface {
	Enabled() bool
	// Reminder devuelve la plantilla de recordatorio y su idioma.
	Reminder() (template, lang string)
	// SendTemplate envía la plantilla name en el idioma lang al número to (solo dígitos, con lada de país) con los parámetros del cuerpo en orden.
	SendTemplate(ctx context.Context, to, name, lang string, params []string) error
}

// Cloud es el remitente real.
type Cloud struct {
	Token    string // token de acceso permanente (usuario del sistema)
	PhoneID  string // identificador del número de WhatsApp Business
	Template string // plantilla de recordatorio aprobada
	Lang     string // idioma de la plantilla, por ejemplo es_MX
	Base     string // para pruebas; vacío usa la oficial
	HTTP     *http.Client
}

const defaultBase = "https://graph.facebook.com/v21.0"

// Reminder devuelve la plantilla de recordatorio aprobada y su idioma (es_MX por omisión).
func (c *Cloud) Reminder() (string, string) {
	lang := c.Lang
	if lang == "" {
		lang = "es_MX"
	}
	return c.Template, lang
}

func (c *Cloud) Enabled() bool {
	return c != nil && c.Token != "" && c.PhoneID != "" && c.Template != ""
}

// clean cumple las reglas de WhatsApp para un parámetro: sin saltos de línea ni tabuladores ni más de 4 espacios seguidos.
func clean(v string) string {
	v = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(v)
	v = strings.Join(strings.Fields(v), " ")
	if v == "" {
		return "-" // un parámetro vacío hace que Meta rechace el mensaje
	}
	if r := []rune(v); len(r) > 200 {
		v = string(r[:200])
	}
	return v
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (c *Cloud) SendTemplate(ctx context.Context, to, name, lang string, params []string) error {
	if !c.Enabled() {
		return errors.New("whatsapp: no configurado")
	}
	type param struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	ps := make([]param, len(params))
	for i, p := range params {
		ps[i] = param{"text", clean(p)}
	}
	body, _ := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template": map[string]any{
			"name":       name,
			"language":   map[string]string{"code": lang},
			"components": []any{map[string]any{"type": "body", "parameters": ps}},
		},
	})
	base := strings.TrimRight(c.Base, "/")
	if base == "" {
		base = defaultBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/"+c.PhoneID+"/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if res.StatusCode/100 != 2 {
		var e apiError
		if json.Unmarshal(raw, &e) == nil && e.Error.Message != "" {
			return fmt.Errorf("whatsapp: %d (código %d): %s", res.StatusCode, e.Error.Code, e.Error.Message)
		}
		return fmt.Errorf("whatsapp: respuesta %d", res.StatusCode)
	}
	return nil
}
