package main

import (
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "session"
const sessionTTL = 7 * 24 * time.Hour

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *server) setSession(w http.ResponseWriter, uid int64) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(uid, 10),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(sessionTTL)),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// auth valida o cookie JWT e injeta o id do usuário no handler.
func (s *server) auth(next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			writeErr(w, 401, "não autenticado")
			return
		}
		claims := &jwt.RegisteredClaims{}
		tok, err := jwt.ParseWithClaims(c.Value, claims, func(t *jwt.Token) (any, error) { return s.jwtSecret, nil },
			jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !tok.Valid {
			writeErr(w, 401, "sessão inválida")
			return
		}
		uid, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			writeErr(w, 401, "sessão inválida")
			return
		}
		next(w, r, uid)
	}
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow("reg:"+clientIP(r), 10, time.Hour) {
		writeErr(w, 429, "muitas tentativas, tente mais tarde")
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	c.Name = strings.TrimSpace(c.Name)
	addr, err := mail.ParseAddress(strings.TrimSpace(c.Email))
	switch {
	case utf8.RuneCountInString(c.Name) < 2 || utf8.RuneCountInString(c.Name) > 80:
		writeErr(w, 400, "informe um nome (2 a 80 caracteres)")
		return
	case err != nil || len(addr.Address) > 254:
		writeErr(w, 400, "email inválido")
		return
	case len(c.Password) < 8 || len(c.Password) > 72:
		writeErr(w, 400, "a senha deve ter de 8 a 72 caracteres")
		return
	}
	email := strings.ToLower(addr.Address)
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "erro interno")
		return
	}
	res, err := s.db.Exec(`INSERT INTO users(name,email,password_hash) VALUES(?,?,?)`, c.Name, email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeErr(w, 409, "este email já está cadastrado")
			return
		}
		writeErr(w, 500, "erro interno")
		return
	}
	id, _ := res.LastInsertId()
	s.setSession(w, id)
	writeJSON(w, 201, map[string]any{"id": id, "name": c.Name, "email": email})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(c.Email))
	// limita por IP e por email (mitiga força bruta)
	if !s.limiter.allow("login-ip:"+clientIP(r), 20, 10*time.Minute) || !s.limiter.allow("login-em:"+email, 8, 10*time.Minute) {
		writeErr(w, 429, "muitas tentativas, aguarde alguns minutos")
		return
	}
	var id int64
	var name, hash string
	err := s.db.QueryRow(`SELECT id,name,password_hash FROM users WHERE email=?`, email).Scan(&id, &name, &hash)
	if err != nil {
		bcrypt.CompareHashAndPassword(s.dummyHash, []byte(c.Password)) // tempo constante
		writeErr(w, 401, "email ou senha incorretos")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		writeErr(w, 401, "email ou senha incorretos")
		return
	}
	s.setSession(w, id)
	writeJSON(w, 200, map[string]any{"id": id, "name": name, "email": email})
}

func (s *server) me(w http.ResponseWriter, r *http.Request, uid int64) {
	var name, email string
	if err := s.db.QueryRow(`SELECT name,email FROM users WHERE id=?`, uid).Scan(&name, &email); err != nil {
		writeErr(w, 401, "usuário não encontrado")
		return
	}
	writeJSON(w, 200, map[string]any{"id": uid, "name": name, "email": email})
}
