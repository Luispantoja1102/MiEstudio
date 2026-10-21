package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Principal es la persona autenticada. El rol y el negocio se leen de la base
// en cada petición, así un cambio de rol o una baja surten efecto de inmediato.
type Principal struct {
	UserID         int64
	BusinessID     int64 // 0 para usuarios de plataforma
	Role           string
	BusinessStatus string // vacío para usuarios de plataforma
	SessionVersion int    // un token solo vale si lleva esta misma versión
}

// Roles de plataforma (sin negocio).
const (
	RolePlatformAdmin   = "platform_admin"
	RolePlatformSupport = "platform_support"
)

func (p *Principal) IsPlatform() bool {
	return p.Role == RolePlatformAdmin || p.Role == RolePlatformSupport
}

// Lookup devuelve nil, nil si el usuario no existe o está inactivo.
type Lookup func(ctx context.Context, userID int64) (*Principal, error)

type claims struct {
	UserID int64 `json:"uid"`
	SV     int   `json:"sv"`
	// Purpose vacío = sesión. Un token con propósito (p. ej. "2fa") no abre ninguna ruta.
	Purpose string `json:"pur,omitempty"`
	jwt.RegisteredClaims
}

func HashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

// SessionHours es lo que dura una sesión.
const SessionHours = 12

func Sign(secret string, userID int64, sessionVersion int) (string, error) {
	c := claims{
		UserID:           userID,
		SV:               sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(SessionHours * time.Hour))},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

func parseToken(secret, token string) (uid int64, sv int, err error) {
	c := &claims{}
	t, err := jwt.ParseWithClaims(token, c, func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid || c.UserID == 0 || c.Purpose != "" {
		return 0, 0, errors.New("token inválido")
	}
	return c.UserID, c.SV, nil
}

// CookieName es la cookie de sesión: HttpOnly, así ningún script de la página puede leerla.
const CookieName = "miestudio_session"

// CSRFHeader debe acompañar a toda petición que cambie datos y se autentique con la cookie.
// Un formulario de otro sitio no puede añadir encabezados propios sin permiso del navegador.
const CSRFHeader = "X-Requested-With"
const CSRFValue = "miestudio"

func SetCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", MaxAge: SessionHours * 3600,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

// tokenFrom busca el token: encabezado Bearer (herramientas y pruebas) o la cookie (el navegador).
func tokenFrom(r *http.Request) (token string, fromCookie bool) {
	if t, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return t, false
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value, true
	}
	return "", false
}

type ctxKey struct{}

func deny(w http.ResponseWriter, status int, msg string) { denyCode(w, status, msg, "") }

func denyCode(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]string{"error": msg}
	if code != "" {
		body["code"] = code
	}
	json.NewEncoder(w).Encode(body)
}

// Middleware exige un token Bearer válido y un usuario activo.
func Middleware(secret string, lookup Lookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, fromCookie := tokenFrom(r)
			if token == "" {
				deny(w, http.StatusUnauthorized, "no autenticado")
				return
			}
			if fromCookie && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions &&
				r.Header.Get(CSRFHeader) != CSRFValue {
				denyCode(w, http.StatusForbidden, "solicitud no permitida", "csrf")
				return
			}
			uid, sv, err := parseToken(secret, token)
			if err != nil {
				deny(w, http.StatusUnauthorized, "no autenticado")
				return
			}
			p, err := lookup(r.Context(), uid)
			if err != nil {
				deny(w, http.StatusInternalServerError, "error interno")
				return
			}
			if p == nil || p.SessionVersion != sv { // baja, o la sesión se cerró (cambio de clave, de rol…)
				deny(w, http.StatusUnauthorized, "no autenticado")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
		})
	}
}

// RequireRole debe ir después de Middleware.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p := FromContext(r.Context()); p == nil || !slices.Contains(roles, p.Role) {
				deny(w, http.StatusForbidden, "no tienes permiso para esto")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireTenant deja pasar solo a usuarios de un negocio cuya cuenta esté en uso
// (en prueba o activa). Debe ir después de Middleware.
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := FromContext(r.Context())
		switch {
		case p == nil || p.IsPlatform() || p.BusinessID == 0:
			deny(w, http.StatusForbidden, "esta sección es solo para negocios")
		case p.BusinessStatus == "pending":
			denyCode(w, http.StatusForbidden, "tu cuenta está pendiente de aprobación", "business_pending")
		case p.BusinessStatus == "suspended":
			denyCode(w, http.StatusForbidden, "tu cuenta está suspendida, contáctanos", "business_suspended")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}

// SignPurpose firma un token de un solo propósito y vida corta (por ejemplo, el reto del segundo paso).
// Nunca sirve como sesión: el middleware solo acepta tokens sin propósito.
func SignPurpose(secret, purpose string, userID int64, ttl time.Duration) (string, error) {
	c := claims{UserID: userID, Purpose: purpose, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl))}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

// ParsePurpose devuelve la persona del token si es del propósito indicado y no venció.
func ParsePurpose(secret, purpose, token string) (int64, error) {
	c := &claims{}
	t, err := jwt.ParseWithClaims(token, c, func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid || c.UserID == 0 || c.Purpose != purpose {
		return 0, errors.New("token inválido")
	}
	return c.UserID, nil
}
