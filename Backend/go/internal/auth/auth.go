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
	jwt.RegisteredClaims
}

func HashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

func Sign(secret string, userID int64) (string, error) {
	c := claims{
		UserID:           userID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour))},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

func parseUserID(secret, token string) (int64, error) {
	c := &claims{}
	t, err := jwt.ParseWithClaims(token, c, func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid || c.UserID == 0 {
		return 0, errors.New("token inválido")
	}
	return c.UserID, nil
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
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				deny(w, http.StatusUnauthorized, "no autenticado")
				return
			}
			uid, err := parseUserID(secret, token)
			if err != nil {
				deny(w, http.StatusUnauthorized, "no autenticado")
				return
			}
			p, err := lookup(r.Context(), uid)
			if err != nil {
				deny(w, http.StatusInternalServerError, "error interno")
				return
			}
			if p == nil {
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
