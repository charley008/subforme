package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	sessionCookieName = "subforme_session"
	sessionMaxAge     = 30 * 24 * 60 * 60
)

type Service struct {
	mu       sync.RWMutex
	Username string
	Password string
}

func (s *Service) Check(username, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return username == s.Username && password == s.Password
}

func (s *Service) UpdatePassword(newPassword string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Password = newPassword
}

// Sessions are scoped to one server instance. Restarting invalidates all tokens.
// Only token hashes are retained; a known legacy signing secret cannot mint one.
type Sessions struct {
	mu     sync.Mutex
	tokens map[[32]byte]time.Time
	now    func() time.Time
}

func NewSessions() *Sessions {
	return &Sessions{tokens: make(map[[32]byte]time.Time), now: time.Now}
}

func (s *Sessions) Set(w http.ResponseWriter, r *http.Request) error {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	value := hex.EncodeToString(token[:])
	now := s.now()
	expires := now.Add(time.Duration(sessionMaxAge) * time.Second)
	s.mu.Lock()
	for key, expiry := range s.tokens {
		if !now.Before(expiry) {
			delete(s.tokens, key)
		}
	}
	s.tokens[sha256.Sum256([]byte(value))] = expires
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/",
		MaxAge: sessionMaxAge, Expires: expires,
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Sessions) Has(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || len(cookie.Value) != 64 {
		return false
	}
	key := sha256.Sum256([]byte(cookie.Value))
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.tokens[key]
	if !ok {
		return false
	}
	if !s.now().Before(expires) {
		delete(s.tokens, key)
		return false
	}
	return true
}

func (s *Sessions) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.mu.Lock()
		delete(s.tokens, sha256.Sum256([]byte(cookie.Value)))
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Sessions) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.tokens)
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
