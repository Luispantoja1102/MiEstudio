package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendTemplateBuildsTheCloudAPIRequest(t *testing.T) {
	var path, auth string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &body)
		w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer srv.Close()
	c := &Cloud{Token: "tok", PhoneID: "123", Template: "recordatorio_cita", Lang: "es_MX", Base: srv.URL}
	if err := c.SendTemplate(context.Background(), "5216641234567", "recordatorio_cita", "es_MX", []string{"Laura", "Corte\ncon  tijeras", ""}); err != nil {
		t.Fatal(err)
	}
	if path != "/123/messages" || auth != "Bearer tok" || body["to"] != "5216641234567" || body["type"] != "template" || body["messaging_product"] != "whatsapp" {
		t.Fatalf("petición: %s %s %v", path, auth, body)
	}
	tpl := body["template"].(map[string]any)
	if tpl["name"] != "recordatorio_cita" || tpl["language"].(map[string]any)["code"] != "es_MX" {
		t.Fatalf("plantilla: %v", tpl)
	}
	params := tpl["components"].([]any)[0].(map[string]any)["parameters"].([]any)
	got := []string{}
	for _, p := range params {
		got = append(got, p.(map[string]any)["text"].(string))
	}
	if strings.Join(got, "|") != "Laura|Corte con tijeras|-" {
		t.Fatalf("parámetros: %v", got)
	}
}

func TestSendTemplateReportsMetaErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"Template name does not exist","code":132001}}`))
	}))
	defer srv.Close()
	c := &Cloud{Token: "t", PhoneID: "1", Template: "x", Base: srv.URL}
	err := c.SendTemplate(context.Background(), "52664", "x", "es_MX", nil)
	if err == nil || !strings.Contains(err.Error(), "132001") || !strings.Contains(err.Error(), "Template name does not exist") {
		t.Fatalf("error: %v", err)
	}
	if (&Cloud{}).Enabled() || (*Cloud)(nil).Enabled() {
		t.Fatal("sin configuración no debe estar activo")
	}
	if err := (&Cloud{}).SendTemplate(context.Background(), "1", "x", "es", nil); err == nil {
		t.Fatal("sin configuración debe fallar")
	}
}
