package security

import (
	"net/http"
	"time"
)

const RefreshCookieName = "refresh_token"

func SetRefreshToken(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	name := RefreshCookieName
	if secure {
		name = "__Host-" + RefreshCookieName
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/", // 覆盖整站，以便 BFF 转发
		HttpOnly: true,
		Secure:   secure, // prod=true, dev=false
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func ClearRefreshToken(w http.ResponseWriter, secure bool) {
	name := RefreshCookieName
	if secure {
		name = "__Host-" + RefreshCookieName
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func ReadRefreshToken(r *http.Request) (string, error) {
	// 优先尝试读取安全 Cookie
	if c, err := r.Cookie("__Host-" + RefreshCookieName); err == nil {
		return c.Value, nil
	}
	// Fallback (主要用于本地非 HTTPS 开发环境)
	c, err := r.Cookie(RefreshCookieName)
	if err != nil {
		return "", err
	}
	return c.Value, nil
}
