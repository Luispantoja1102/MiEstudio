package mail

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBuildEncodesAndBlocksHeaderInjection(t *testing.T) {
	raw := string(Build("A <a@x.com>", "b@x.com", Message{Subject: "Cita ñandú\r\nBcc: evil@x.com", Text: "hola ñ", HTML: "<p>hola</p>"}))
	if strings.Contains(raw, "\r\nBcc:") {
		t.Fatalf("inyección de encabezados:\n%s", raw)
	}
	if !strings.Contains(raw, "multipart/alternative") || !strings.Contains(raw, "text/html") || !strings.Contains(raw, "=C3=B1") {
		t.Fatalf("estructura inesperada:\n%s", raw)
	}
	if !strings.Contains(raw, "=?utf-8?q?") {
		t.Fatalf("asunto sin codificar:\n%s", raw)
	}
}

func TestEnabled(t *testing.T) {
	var nilSMTP *SMTP
	if nilSMTP.Enabled() || (&SMTP{Host: "h"}).Enabled() || !(&SMTP{Host: "h", From: "a@x.com"}).Enabled() {
		t.Fatal("Enabled mal calculado")
	}
	if err := (&SMTP{}).Send(context.Background(), Message{To: "a@x.com"}); err == nil {
		t.Fatal("sin configurar debe fallar")
	}
}

// Servidor SMTP mínimo en claro: comprueba el diálogo completo y que no se manden credenciales sin cifrar.
func fakeSMTP(t *testing.T, got chan string) (host string, port int) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		say := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
		say("220 fake ESMTP")
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
				got <- strings.TrimSpace(line)
				say("250 ok")
			case cmd == "DATA":
				say("354 go")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					data.WriteString(l)
				}
				got <- data.String()
				say("250 queued")
			case cmd == "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	a := l.Addr().(*net.TCPAddr)
	return "127.0.0.1", a.Port
}

func TestSendPlainSMTP(t *testing.T) {
	got := make(chan string, 8)
	host, port := fakeSMTP(t, got)
	s := &SMTP{Host: host, Port: port, From: "MiEstudio <no-reply@x.com>"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Send(ctx, Message{To: "Laura <laura@x.com>", Subject: "Hola", Text: "texto"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"no-reply@x.com", "laura@x.com", "Subject:"} {
		if v := <-got; !strings.Contains(v, want) {
			t.Fatalf("esperaba %q en %q", want, v)
		}
	}
}

func TestSendRefusesCredentialsWithoutTLS(t *testing.T) {
	got := make(chan string, 8)
	host, port := fakeSMTP(t, got)
	s := &SMTP{Host: host, Port: port, From: "a@x.com", User: "u", Pass: "p"}
	err := s.Send(context.Background(), Message{To: "b@x.com", Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("debía negarse a enviar credenciales en claro: %v", err)
	}
}

func TestSendRejectsBadAddresses(t *testing.T) {
	s := &SMTP{Host: "h", From: "a@x.com"}
	if err := s.Send(context.Background(), Message{To: "no es correo"}); err == nil {
		t.Fatal("destinatario inválido debe fallar")
	}
}

func TestBuildAddsReplyToOnlyWhenValid(t *testing.T) {
	raw := string(Build("Barbella <no-reply@x.mx>", "equipo@x.mx", Message{Subject: "Hola", Text: "hola", ReplyTo: "Ana López <ana@correo.com>"}))
	if !strings.Contains(raw, "Reply-To: ") || !strings.Contains(raw, "ana@correo.com") {
		t.Fatalf("falta Reply-To:\n%s", raw)
	}
	for _, bad := range []string{"", "no es un correo"} {
		if raw := string(Build("a@x.mx", "b@x.mx", Message{Subject: "Hola", Text: "hola", ReplyTo: bad})); strings.Contains(raw, "Reply-To") {
			t.Fatalf("ReplyTo %q no debe producir encabezado:\n%s", bad, raw)
		}
	}
	// un valor con salto de línea no puede inyectar otro encabezado
	if raw := string(Build("a@x.mx", "b@x.mx", Message{Subject: "Hola", Text: "hola", ReplyTo: "ana@correo.com\r\nBcc: otro@x.mx"})); strings.Contains(raw, "\r\nBcc:") {
		t.Fatalf("inyección de encabezado:\n%s", raw)
	}
}
