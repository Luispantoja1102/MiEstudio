package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

// Detrás de un proxy (Nginx, Caddy, Cloudflare…) la conexión directa siempre viene del proxy, y los límites
// de intentos creerían que todo el mundo es la misma persona. Si el proxy está declarado como de confianza
// (TRUSTED_PROXIES), la dirección real se toma de X-Forwarded-For. Un encabezado que llega de cualquier otro
// lado se ignora: de lo contrario cualquiera podría falsear su IP para esquivar los límites.

var trustedProxies atomic.Pointer[[]*net.IPNet]

// SetTrustedProxies recibe direcciones o rangos separados por comas ("127.0.0.1, 10.0.0.0/8"). Vacío = ninguno.
func SetTrustedProxies(spec string) error {
	var nets []*net.IPNet
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			ip := net.ParseIP(part)
			if ip == nil {
				return fmt.Errorf("TRUSTED_PROXIES: %q no es una dirección ni un rango válido", part)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			part = fmt.Sprintf("%s/%d", ip, bits)
		}
		_, n, err := net.ParseCIDR(part)
		if err != nil {
			return fmt.Errorf("TRUSTED_PROXIES: %q no es una dirección ni un rango válido", part)
		}
		nets = append(nets, n)
	}
	trustedProxies.Store(&nets)
	return nil
}

func isTrusted(ip net.IP) bool {
	p := trustedProxies.Load()
	if p == nil || ip == nil {
		return false
	}
	for _, n := range *p {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// clientIP es la dirección de quien hace la petición, descontando los proxies de confianza.
func clientIP(r *http.Request) string {
	host := remoteHost(r)
	if !isTrusted(net.ParseIP(host)) {
		return host
	}
	// de derecha a izquierda: cada proxy añade la dirección de quien le habló; la primera que no es de
	// confianza es el cliente. Lo que esté más a la izquierda puede venir falseado por el propio cliente.
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			break
		}
		if !isTrusted(ip) {
			return ip.String()
		}
	}
	return host
}

// isHTTPS dice si el visitante llegó por https (directo, o a través de un proxy de confianza que lo avisa).
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return isTrusted(net.ParseIP(remoteHost(r))) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// secureCookie: la cookie de sesión solo viaja por https en producción o cuando la conexión ya es https.
func (a *API) secureCookie(r *http.Request) bool { return a.Production || isHTTPS(r) }
