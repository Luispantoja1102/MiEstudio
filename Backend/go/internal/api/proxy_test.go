package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func req(remote string, xff ...string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	return r
}

func TestClientIPTrustsOnlyDeclaredProxies(t *testing.T) {
	t.Cleanup(func() { SetTrustedProxies("") })
	SetTrustedProxies("")
	if got := clientIP(req("203.0.113.9:1234", "1.2.3.4")); got != "203.0.113.9" {
		t.Fatalf("sin proxies de confianza se ignora X-Forwarded-For: %s", got)
	}
	if err := SetTrustedProxies("10.0.0.0/8, 127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		r    *http.Request
		want string
	}{
		{"proxy de confianza", req("10.0.0.5:80", "198.51.100.7"), "198.51.100.7"},
		{"dos proxies encadenados", req("10.0.0.5:80", "198.51.100.7, 10.0.0.9"), "198.51.100.7"},
		{"el cliente falsea la izquierda", req("10.0.0.5:80", "9.9.9.9, 198.51.100.7"), "198.51.100.7"},
		{"cabeceras repetidas", req("10.0.0.5:80", "9.9.9.9", "198.51.100.7"), "198.51.100.7"},
		{"origen que no es proxy", req("203.0.113.9:1", "1.2.3.4"), "203.0.113.9"},
		{"cabecera basura", req("127.0.0.1:1", "no-es-ip"), "127.0.0.1"},
		{"sin cabecera", req("127.0.0.1:1"), "127.0.0.1"},
		{"ipv6", req("127.0.0.1:1", "2001:db8::1"), "2001:db8::1"},
	}
	for _, c := range cases {
		if got := clientIP(c.r); got != c.want {
			t.Errorf("%s: %s, esperaba %s", c.name, got, c.want)
		}
	}
	for _, bad := range []string{"no-ip", "10.0.0.0/99"} {
		if SetTrustedProxies(bad) == nil {
			t.Errorf("%q debía rechazarse", bad)
		}
	}
}

func TestIsHTTPSOnlyFromTrustedProxy(t *testing.T) {
	t.Cleanup(func() { SetTrustedProxies("") })
	SetTrustedProxies("127.0.0.1")
	r := req("127.0.0.1:1")
	r.Header.Set("X-Forwarded-Proto", "https")
	if !isHTTPS(r) {
		t.Fatal("un proxy de confianza puede avisar https")
	}
	r2 := req("203.0.113.9:1")
	r2.Header.Set("X-Forwarded-Proto", "https")
	if isHTTPS(r2) {
		t.Fatal("un origen cualquiera no puede declarar https")
	}
}
