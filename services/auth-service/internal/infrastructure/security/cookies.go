package security

import (
	"net/http"
	"time"
)

const RefreshCookieName = "refresh_token"

func SetRefreshToken(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/auth/v1", // 限制作用域
		HttpOnly: true,
		Secure:   secure, // prod=true, dev=false
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func ClearRefreshToken(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/auth/v1",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func ReadRefreshToken(r *http.Request) (string, error) {
	c, err := r.Cookie(RefreshCookieName)
	if err != nil {
		return "", err
	}
	return c.Value, nil
}
