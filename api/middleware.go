package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sessionCookie = "rf_session"

// sessionToken 用 HMAC-SHA256 生成无状态 session token
func sessionToken(password string) string {
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte("ruleflow:authenticated"))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateSession 校验请求是否携带有效 session
func ValidateSession(r *http.Request, password string) bool {
	if password == "" {
		return true
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	expected := sessionToken(password)
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(expected)) == 1
}

// SetSessionCookie 在响应中写入 session cookie
func SetSessionCookie(w http.ResponseWriter, password string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionToken(password),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 30, // 30 天
	})
}

// ClearSessionCookie 清除 session cookie（退出登录）
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// WebAuthMiddleware Web 页面鉴权：未登录时重定向到登录页
func WebAuthMiddleware(password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ValidateSession(r, password) {
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// APIAuthMiddleware API 鉴权：未登录时返回 401 JSON
func APIAuthMiddleware(password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ValidateSession(r, password) {
				SendError(w, http.StatusUnauthorized, "未登录，请先访问 /login")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 创建响应写入器包装器以捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// 调用下一个处理器
		next.ServeHTTP(wrapped, r)

		// 记录日志
		duration := time.Since(start)
		log.Printf("%s %s %d %v",
			r.Method,
			r.URL.Path,
			wrapped.status,
			duration,
		)
	})
}

// CORSMiddleware CORS 中间件
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Add("Vary", "Origin")

				allowedOrigin, allowCredentials := resolveAllowedOrigin(origin, allowedOrigins)
				if allowedOrigin == "" {
					next.ServeHTTP(w, r)
					return
				}

				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func resolveAllowedOrigin(origin, allowedOrigins string) (string, bool) {
	for _, allowedOrigin := range strings.Split(allowedOrigins, ",") {
		allowedOrigin = strings.TrimSpace(allowedOrigin)
		if allowedOrigin == "" {
			continue
		}
		if allowedOrigin == "*" {
			return "*", false
		}
		if allowedOrigin == origin {
			return origin, true
		}
	}
	return "", false
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				SendError(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// responseWriter 响应写入器包装器
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}
