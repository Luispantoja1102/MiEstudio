// Package mail envía correo por SMTP (STARTTLS en 587 o TLS directo en 465).
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Message es un correo con versión de texto y, opcionalmente, HTML.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
	ReplyTo string // opcional: a quién llega la respuesta (por ejemplo, la persona que se registró)
	// Attachments son archivos adjuntos (por ejemplo, el .ics de una cita).
	Attachments []Attachment
}

// Attachment es un archivo adjunto de un correo.
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// Sender entrega correos. La API depende de esta interfaz para poder capturarlos en las pruebas.
type Sender interface {
	Enabled() bool
	Send(ctx context.Context, m Message) error
}

// SMTP es el remitente real.
type SMTP struct {
	Host   string
	Port   int
	User   string
	Pass   string
	From   string // «Nombre <correo>» o solo el correo
	Secure bool   // TLS directo (puerto 465); si no, se usa STARTTLS cuando el servidor lo ofrece
}

// Enabled dice si hay lo mínimo para enviar.
func (s *SMTP) Enabled() bool { return s != nil && s.Host != "" && s.From != "" }

// clean quita saltos de línea: evita que un dato externo inyecte encabezados.
func clean(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(v))
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	if !s.Enabled() {
		return errors.New("mail: SMTP no configurado")
	}
	from, err := mail.ParseAddress(clean(s.From))
	if err != nil {
		return fmt.Errorf("mail: remitente inválido: %w", err)
	}
	to, err := mail.ParseAddress(clean(m.To))
	if err != nil {
		return fmt.Errorf("mail: destinatario inválido: %w", err)
	}
	raw := Build(from.String(), to.String(), m)

	port := s.Port
	if port == 0 {
		port = 587
		if s.Secure {
			port = 465
		}
	}
	addr := net.JoinHostPort(s.Host, fmt.Sprint(port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	var conn net.Conn
	if s.Secure {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mail: conexión: %w", err)
	}
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: saludo: %w", err)
	}
	defer c.Close()
	if !s.Secure {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
		} else if s.User != "" {
			return errors.New("mail: el servidor no ofrece STARTTLS; no se envían credenciales sin cifrar")
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Pass, s.Host)); err != nil {
			return fmt.Errorf("mail: autenticación: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("mail: remitente: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mail: destinatario: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: datos: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("mail: escritura: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: cierre: %w", err)
	}
	return c.Quit()
}

// Build arma el mensaje: multipart/alternative con partes quoted-printable y, si hay adjuntos, dentro de un multipart/mixed.
func Build(from, to string, m Message) []byte {
	stamp := time.Now().UnixNano()
	altBoundary := fmt.Sprintf("miestudio-%d", stamp)
	mixBoundary := fmt.Sprintf("miestudio-mix-%d", stamp)
	var b bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", from)
	h("To", to)
	if r, err := mail.ParseAddress(clean(m.ReplyTo)); err == nil {
		h("Reply-To", r.String())
	}
	h("Subject", mime.QEncoding.Encode("utf-8", clean(m.Subject)))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")

	// el cuerpo (solo texto, o texto y HTML); con adjuntos se escribe dentro de un multipart/mixed
	var body bytes.Buffer
	qp := func(s string) {
		q := quotedprintable.NewWriter(&body)
		_, _ = q.Write([]byte(s))
		_ = q.Close()
	}
	var bodyType string
	if m.HTML == "" {
		bodyType = "text/plain; charset=UTF-8"
		qp(m.Text)
	} else {
		bodyType = fmt.Sprintf(`multipart/alternative; boundary="%s"`, altBoundary)
		part := func(ctype, text string) {
			fmt.Fprintf(&body, "--%s\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", altBoundary, ctype)
			qp(text)
			body.WriteString("\r\n")
		}
		part("text/plain", m.Text)
		part("text/html", m.HTML)
		fmt.Fprintf(&body, "--%s--\r\n", altBoundary)
	}

	if len(m.Attachments) == 0 {
		h("Content-Type", bodyType)
		if m.HTML == "" {
			h("Content-Transfer-Encoding", "quoted-printable")
		}
		b.WriteString("\r\n")
		b.Write(body.Bytes())
		return b.Bytes()
	}

	h("Content-Type", fmt.Sprintf(`multipart/mixed; boundary="%s"`, mixBoundary))
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\nContent-Type: %s\r\n", mixBoundary, bodyType)
	if m.HTML == "" {
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	}
	b.WriteString("\r\n")
	b.Write(body.Bytes())
	b.WriteString("\r\n")
	for _, a := range m.Attachments {
		name := clean(a.Name)
		ctype := clean(a.ContentType)
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; name=%q\r\nContent-Transfer-Encoding: base64\r\nContent-Disposition: attachment; filename=%q\r\n\r\n", mixBoundary, ctype, name, name)
		enc := base64.StdEncoding.EncodeToString(a.Data)
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", mixBoundary)
	return b.Bytes()
}
