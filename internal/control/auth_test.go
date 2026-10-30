package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func loginBody(password string) *bytes.Reader {
	body, _ := json.Marshal(map[string]string{"password": password})
	return bytes.NewReader(body)
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/api/login", loginBody(testPassword)))
	if w.Code != 200 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("login failed: %d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != int(sessionLifetime.Seconds()) {
		t.Fatal("session cookie is missing required attributes")
	}
	if bytes.Contains(w.Body.Bytes(), []byte(cookie.Value)) {
		t.Fatal("session token exposed in JSON")
	}
	return cookie
}

func TestSessionProtectsFaultsAndLogoutRevokesAccess(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler(http.NotFoundHandler())
	cookie := loginCookie(t, h)
	for _, test := range []struct {
		path, body string
		cookie     *http.Cookie
		status     int
	}{
		{"fault/license", "{}", nil, 401},
		{"traffic", `{"rate":81}`, cookie, 400},
		{"traffic", `{"rate":22}`, cookie, 200},
		{"fault/license", "{}", cookie, 202},
		{"logout", "{}", cookie, 200},
		{"fault/license", "{}", cookie, 401},
	} {
		r := httptest.NewRequest("POST", "/api/"+test.path, bytes.NewBufferString(test.body))
		if test.cookie != nil {
			r.AddCookie(test.cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Errorf("%s: expected %d got %d", test.path, test.status, w.Code)
		}
	}
	if s.store.(*fakeStore).invalidations != 1 {
		t.Fatal("unauthorized invalidation executed")
	}
}

func TestInvalidExpiredAndPreviousSessionsAreRejected(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler(http.NotFoundHandler())
	cookie := loginCookie(t, h)
	s.auth.sessions[cookie.Value] = time.Now().Add(-time.Second)
	for _, value := range []string{cookie.Value, "forged-session"} {
		r := httptest.NewRequest("GET", "/api/state", nil)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
		r.Header.Set("Authorization", "Bearer "+testPassword)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("invalid session accepted")
		}
	}
	cookie = loginCookie(t, h)
	r := httptest.NewRequest("GET", "/api/state", nil)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	newTestServer(t).Handler(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("session survived server restart")
	}
}

func TestBrowserCrossOriginRequestsCannotLogInOrInjectFaults(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler(http.NotFoundHandler())
	cookie := loginCookie(t, h)
	for _, path := range []string{"login", "fault/license", "logout"} {
		r := httptest.NewRequest("POST", "/api/"+path, loginBody(testPassword))
		r.AddCookie(cookie)
		r.Header.Set("Origin", "https://other.example")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("cross-origin %s accepted", path)
		}
	}
	if s.store.(*fakeStore).invalidations != 0 {
		t.Fatal("cross-origin invalidation executed")
	}
}

func TestIncorrectPasswordsAreRateLimited(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler(http.NotFoundHandler())
	for attempt := 0; attempt < 11; attempt++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/api/login", loginBody(testPassword+"-incorrect")))
		expected := 401
		if attempt == 10 {
			expected = 429
		}
		if w.Code != expected || len(w.Result().Cookies()) != 0 {
			t.Fatal("incorrect password did not fail safely")
		}
	}
	s.auth.window = time.Now().Add(-time.Minute)
	s.auth.secure = true
	if !loginCookie(t, h).Secure {
		t.Fatal("HTTPS cookie setting was ignored")
	}
}
