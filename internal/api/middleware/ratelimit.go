package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/logan/cloudcode/internal/api/response"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit returns middleware that limits requests per IP.
// rps is the steady-state rate (requests per second), burst is the max burst size.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	var mu sync.Mutex
	limiters := make(map[string]*ipLimiter)

	// Cleanup stale entries every 3 minutes
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			mu.Lock()
			for ip, l := range limiters {
				if time.Since(l.lastSeen) > 5*time.Minute {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			l, ok := limiters[ip]
			if !ok {
				l = &ipLimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
				limiters[ip] = l
			}
			l.lastSeen = time.Now()
			mu.Unlock()

			if !l.limiter.Allow() {
				w.Header().Set("Retry-After", "60")
				response.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
