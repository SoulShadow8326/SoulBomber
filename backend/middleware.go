package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	requests map[string][]time.Time
	mu sync.RWMutex
	limit int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit: limit,
		window: window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	if requests, exists := rl.requests[key]; exists {
		var validRequests []time.Time
		for _, req := range requests {
			if req.After(windowStart) {
				validRequests = append(validRequests, req)
			}
		}
		rl.requests[key] = validRequests

		if len(validRequests) >= rl.limit {
			return false
		}
	}

	rl.requests[key] = append(rl.requests[key], now)
	return true
}

func getClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}

func RateLimitMiddleware(limit int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	limiter := NewRateLimiter(limit, window)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)
			if !limiter.Allow(clientIP) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next(w, r)
		}
	}
}

func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next(rw, r)

		duration := time.Since(start)
		logInfo("HTTP Request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", fmt.Sprintf("%d", rw.statusCode),
			"duration", duration.String(),
			"ip", getClientIP(r),
		)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func RecoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logError("Panic recovered", fmt.Errorf("%v", err),
					"method", r.Method,
					"path", r.URL.Path,
					"ip", getClientIP(r),
				)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}
