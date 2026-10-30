package control

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const sessionCookie = "mayday_session"
const sessionLifetime = 8 * time.Hour

type sessionAuth struct {
	mu       sync.Mutex
	password [32]byte
	secure   bool
	sessions map[string]time.Time
	failures int
	window   time.Time
}

func newSessionAuth(password string, secure bool) *sessionAuth {
	return &sessionAuth{password: sha256.Sum256([]byte(password)), secure: secure, sessions: make(map[string]time.Time)}
}

func (a *sessionAuth) cookie(value string, expires time.Time, maxAge int) *http.Cookie {
	return &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge}
}

func (a *sessionAuth) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		respond(w, 400, map[string]string{"error": "Enter a password"})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if now.Sub(a.window) >= time.Minute {
		a.window, a.failures = now, 0
	}
	if a.failures >= 10 {
		w.Header().Set("Retry-After", "60")
		respond(w, 429, map[string]string{"error": "Too many attempts. Try again in a minute."})
		return
	}
	password := sha256.Sum256([]byte(input.Password))
	if subtle.ConstantTimeCompare(password[:], a.password[:]) != 1 {
		a.failures++
		respond(w, 401, map[string]string{"error": "Incorrect password"})
		return
	}
	for token, expires := range a.sessions {
		if !now.Before(expires) {
			delete(a.sessions, token)
		}
	}
	if previous, err := r.Cookie(sessionCookie); err == nil {
		delete(a.sessions, previous.Value)
	}
	if len(a.sessions) >= 64 {
		var oldest string
		for token, expires := range a.sessions {
			if oldest == "" || expires.Before(a.sessions[oldest]) {
				oldest = token
			}
		}
		delete(a.sessions, oldest)
	}
	token, expires := rand.Text(), now.Add(sessionLifetime)
	a.sessions[token] = expires
	http.SetCookie(w, a.cookie(token, expires, int(sessionLifetime.Seconds())))
	respond(w, 200, map[string]bool{"authenticated": true})
}

func (a *sessionAuth) logout(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		delete(a.sessions, cookie.Value)
	}
	a.mu.Unlock()
	http.SetCookie(w, a.cookie("", time.Unix(1, 0), -1))
	respond(w, 200, map[string]bool{"authenticated": false})
}

func (a *sessionAuth) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		cookie, err := r.Cookie(sessionCookie)
		a.mu.Lock()
		valid := false
		if err == nil {
			expires, exists := a.sessions[cookie.Value]
			valid = exists && time.Now().Before(expires)
			if !valid {
				delete(a.sessions, cookie.Value)
			}
		}
		a.mu.Unlock()
		if !valid {
			respond(w, 401, map[string]string{"error": "Sign in to continue"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
