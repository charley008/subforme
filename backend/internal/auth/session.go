package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName = "subforme_session"
	sessionMaxAge     = 30 * 24 * 60 * 60
)

type Service struct {
	Username string
	Password string
}

func (s *Service) Check(username, password string) bool {
	return username == s.Username && password == s.Password
}

func (s *Service) UpdatePassword(newPassword string) {
	s.Password = newPassword
}

func SetSession(w http.ResponseWriter, secret string) {
	expires := time.Now().Add(time.Duration(sessionMaxAge) * time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signSessionValue("ok", secret),
		Path:     "/",
		MaxAge:   sessionMaxAge,
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HasSession(r *http.Request, secret string) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	if secret == "" {
		return cookie.Value == "ok"
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 || parts[0] != "ok" {
		return false
	}
	return hmac.Equal([]byte(parts[1]), []byte(sessionSignature(parts[0], secret)))
}

func signSessionValue(payload, secret string) string {
	if secret == "" {
		return payload
	}
	return payload + "." + sessionSignature(payload, secret)
}

func sessionSignature(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
