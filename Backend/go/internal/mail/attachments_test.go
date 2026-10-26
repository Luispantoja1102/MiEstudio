package mail

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	netmail "net/mail"
	"strings"
	"testing"
)

func TestBuildWithAttachmentKeepsBodyAndFile(t *testing.T) {
	ics := []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n" + strings.Repeat("x", 200))
	raw := Build("Barbella <no-reply@x.mx>", "ana@x.mx", Message{
		Subject: "Cita", Text: "hola ñ", HTML: "<p>hola</p>",
		Attachments: []Attachment{{Name: "cita.ics", ContentType: "text/calendar; charset=UTF-8; method=PUBLISH", Data: ics}},
	})
	msg, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/mixed" {
		t.Fatalf("tipo %q (%v)", mt, err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	body, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if bt, _, _ := mime.ParseMediaType(body.Header.Get("Content-Type")); bt != "multipart/alternative" {
		t.Fatalf("el primer bloque debe ser el cuerpo, no %q", bt)
	}
	file, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if file.FileName() != "cita.ics" || file.Header.Get("Content-Transfer-Encoding") != "base64" {
		t.Fatalf("adjunto mal armado: %q %v", file.FileName(), file.Header)
	}
	got, _ := io.ReadAll(base64Reader(file))
	if !bytes.Equal(got, ics) {
		t.Fatalf("el adjunto cambió: %d bytes", len(got))
	}
	if _, err := mr.NextPart(); err != io.EOF {
		t.Fatalf("sobran partes: %v", err)
	}
}

func TestBuildWithoutAttachmentIsUnchanged(t *testing.T) {
	raw := string(Build("a@x.mx", "b@x.mx", Message{Subject: "Hola", Text: "hola", HTML: "<p>hola</p>"}))
	if strings.Contains(raw, "multipart/mixed") {
		t.Fatalf("sin adjuntos no debe haber multipart/mixed:\n%s", raw)
	}
	plain := string(Build("a@x.mx", "b@x.mx", Message{Subject: "Hola", Text: "hola"}))
	if !strings.Contains(plain, "text/plain") || strings.Contains(plain, "multipart") {
		t.Fatalf("texto simple inesperado:\n%s", plain)
	}
}

func base64Reader(r io.Reader) io.Reader { return base64.NewDecoder(base64.StdEncoding, r) }
