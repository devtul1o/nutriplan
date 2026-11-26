package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

//go:embed data/taco.json
var tacoRaw []byte

type server struct {
	db        *sql.DB
	jwtSecret []byte
	secure    bool   // cookie Secure (true em produção/HTTPS)
	origin    string // origem permitida do frontend (checagem anti-CSRF)
	usdaKey   string
	taco      []Food
	foldNames []string
	limiter   *limiter
	cache     *ttlCache
	dummyHash []byte
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET precisa ter pelo menos 32 caracteres")
	}
	dbPath := env("DB_PATH", "./data/nutri.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		log.Fatal(err)
	}
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	dummy, _ := bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

	s := &server{
		db:        db,
		jwtSecret: []byte(secret),
		secure:    env("COOKIE_SECURE", "true") == "true",
		origin:    strings.TrimRight(os.Getenv("FRONTEND_ORIGIN"), "/"),
		usdaKey:   os.Getenv("USDA_API_KEY"),
		limiter:   &limiter{hits: map[string][]time.Time{}},
		cache:     &ttlCache{m: map[string]cacheEntry{}},
		dummyHash: dummy,
	}
	if err := s.loadTaco(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/auth/me", s.auth(s.me))
	mux.HandleFunc("GET /api/foods/search", s.auth(s.searchFoods))
	mux.HandleFunc("GET /api/plans", s.auth(s.listPlans))
	mux.HandleFunc("POST /api/plans", s.auth(s.createPlan))
	mux.HandleFunc("PUT /api/plans/{id}", s.auth(s.updatePlan))
	mux.HandleFunc("DELETE /api/plans/{id}", s.auth(s.deletePlan))

	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           s.secureHeaders(s.csrfGuard(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Println("API ouvindo em", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, 400, "requisição inválida")
		return false
	}
	return true
}

// ---------- middlewares ----------

func (s *server) secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// Proteção CSRF: cookie SameSite=Lax + exige JSON + confere o Origin.
func (s *server) csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && s.origin != "" && o != s.origin {
				writeErr(w, 403, "origem não permitida")
				return
			}
			if r.Method != http.MethodDelete && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeErr(w, 415, "use application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- rate limit (em memória) ----------

type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (l *limiter) allow(key string, max int, win time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-win)
	h := l.hits[key]
	k := 0
	for _, t := range h {
		if t.After(cut) {
			h[k] = t
			k++
		}
	}
	h = h[:k]
	if len(h) >= max {
		l.hits[key] = h
		return false
	}
	l.hits[key] = append(h, now)
	return true
}

// clientIP: atrás do Vercel + Caddy, o IP real vem em X-Vercel-Forwarded-For.
func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Vercel-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

// ---------- cache simples ----------

type cacheEntry struct {
	v   []Food
	exp time.Time
}
type ttlCache struct {
	mu sync.Mutex
	m  map[string]cacheEntry
}

func (c *ttlCache) get(k string) ([]Food, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	return e.v, true
}
func (c *ttlCache) set(k string, v []Food) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 2000 {
		c.m = map[string]cacheEntry{}
	}
	c.m[k] = cacheEntry{v, time.Now().Add(30 * time.Minute)}
}
