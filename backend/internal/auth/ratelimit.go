package auth

import (
	"net/http"
	"sync"
	"time"

	"oms-backend/internal/api"

	"golang.org/x/time/rate"
)

type LoginRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rate.Limiter
	rate    rate.Limit
	burst   int
}

func NewLoginRateLimiter(r rate.Limit, burst int) *LoginRateLimiter {
	return &LoginRateLimiter{
		clients: make(map[string]*rate.Limiter),
		rate:    r,
		burst:   burst,
	}
}

func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		l.mu.Lock()

		limiter, exists := l.clients[ip]
		if !exists {
			limiter = rate.NewLimiter(l.rate, l.burst)
			l.clients[ip] = limiter
		}

		allowed := limiter.Allow()

		l.mu.Unlock()

		if !allowed {
			w.Header().Set("Retry-After", "60")
			api.WriteError(
				w,
				http.StatusTooManyRequests,
				"RATE_LIMITED",
				"too many login attempts",
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *LoginRateLimiter) Cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			l.mu.Lock()
			l.clients = make(map[string]*rate.Limiter)
			l.mu.Unlock()
		}
	}()
}
