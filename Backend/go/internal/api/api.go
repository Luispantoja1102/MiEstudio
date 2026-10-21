package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/ratelimit"
)

type API struct {
	DB        *pgxpool.Pool
	Secret    string
	Limiter   *ratelimit.Limiter // nil desactiva el límite de intentos
	StaticDir string             // si se define, sirve el frontend compilado
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	authed := auth.Middleware(a.Secret, a.lookup)
	admin := func(h http.HandlerFunc) http.Handler {
		return authed(auth.RequireRole("admin")(h))
	}

	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.Handle("POST /api/auth/password", authed(http.HandlerFunc(a.changePassword)))
	mux.Handle("GET /api/me", authed(http.HandlerFunc(a.me)))

	mux.Handle("GET /api/team", admin(a.listTeam))
	mux.Handle("POST /api/team", admin(a.createTeamMember))
	mux.Handle("PATCH /api/team/{id}", admin(a.updateTeamMember))

	mux.Handle("/api/", http.NotFoundHandler())
	if a.StaticDir != "" {
		mux.Handle("/", spa(a.StaticDir))
	}
	return mux
}

// lookup lee de la base el rol y el negocio actuales del usuario.
func (a *API) lookup(ctx context.Context, userID int64) (*auth.Principal, error) {
	p := &auth.Principal{UserID: userID}
	err := a.DB.QueryRow(ctx, `SELECT business_id, role FROM users WHERE id = $1 AND active`, userID).
		Scan(&p.BusinessID, &p.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

// spa sirve archivos estáticos y devuelve index.html para rutas del cliente.
func spa(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v) == nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
