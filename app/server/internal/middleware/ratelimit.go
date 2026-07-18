package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	apperrors "github.com/sushazhi/fnos-logmanager/internal/errors"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

const maxRateLimitEntries = 10000

type rateLimitStore struct {
	mu    sync.RWMutex
	store map[string]*types.RateLimitRecord
}

func newRateLimitStore() *rateLimitStore {
	return &rateLimitStore{
		store: make(map[string]*types.RateLimitRecord),
	}
}

func (rls *rateLimitStore) get(key string) *types.RateLimitRecord {
	rls.mu.RLock()
	defer rls.mu.RUnlock()
	return rls.store[key]
}

func (rls *rateLimitStore) set(key string, record *types.RateLimitRecord) {
	rls.mu.Lock()
	defer rls.mu.Unlock()
	if len(rls.store) >= maxRateLimitEntries {
		// Evict oldest entry
		for k := range rls.store {
			delete(rls.store, k)
			break
		}
	}
	rls.store[key] = record
}

func (rls *rateLimitStore) cleanup() {
	rls.mu.Lock()
	defer rls.mu.Unlock()
	now := time.Now().UnixMilli()
	for key, record := range rls.store {
		if now > record.ResetTime {
			delete(rls.store, key)
		}
	}
}

var (
	globalRateLimit  = newRateLimitStore()
	apiRateLimitStore = newRateLimitStore()
	sensitiveRateLimit = newRateLimitStore()
)

func init() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			globalRateLimit.cleanup()
			apiRateLimitStore.cleanup()
			sensitiveRateLimit.cleanup()
		}
	}()
}

// RateLimit is the global rate limiting middleware.
func RateLimit(c *gin.Context) {
	cfg := config.Get()
	ip := utils.GetClientIP(c.Request)
	now := time.Now().UnixMilli()

	record := globalRateLimit.get(ip)
	if record == nil {
		record = &types.RateLimitRecord{
			Count:     0,
			ResetTime: now + cfg.RateLimit.WindowMs,
		}
	}

	if now > record.ResetTime {
		record.Count = 1
		record.ResetTime = now + cfg.RateLimit.WindowMs
	} else {
		record.Count++
	}

	globalRateLimit.set(ip, record)

	c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RateLimit.MaxRequests))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, cfg.RateLimit.MaxRequests-record.Count)))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(record.ResetTime, 10))

	if record.Count > cfg.RateLimit.MaxRequests {
		c.Header("Retry-After", strconv.Itoa(int((record.ResetTime-now)/1000)))
		c.Error(apperrors.NewRateLimitError("请求过于频繁，请稍后再试"))
		c.Abort()
		return
	}

	c.Next()
}

// APIRateLimit returns a middleware that applies per-endpoint rate limiting.
func APIRateLimit(maxRequests int, windowMs int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := utils.GetClientIP(c.Request)
		now := time.Now().UnixMilli()

		record := apiRateLimitStore.get(ip)
		if record == nil {
			record = &types.RateLimitRecord{
				Count:     0,
				ResetTime: now + windowMs,
			}
		}

		if now > record.ResetTime {
			record.Count = 1
			record.ResetTime = now + windowMs
		} else {
			record.Count++
		}

		apiRateLimitStore.set(ip, record)

		if record.Count > maxRequests {
			c.Header("Retry-After", strconv.Itoa(int((record.ResetTime-now)/1000)))
			c.Error(apperrors.NewRateLimitError("API请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// SensitiveActionRateLimit returns a middleware that limits sensitive operations per IP+path.
func SensitiveActionRateLimit(maxRequests int, windowMs int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := utils.GetClientIP(c.Request)
		now := time.Now().UnixMilli()
		key := ip + ":" + c.Request.URL.Path

		record := sensitiveRateLimit.get(key)
		if record == nil {
			record = &types.RateLimitRecord{
				Count:     0,
				ResetTime: now + windowMs,
			}
		}

		if now > record.ResetTime {
			record.Count = 1
			record.ResetTime = now + windowMs
		} else {
			record.Count++
		}

		sensitiveRateLimit.set(key, record)

		if record.Count > maxRequests {
			c.Header("Retry-After", strconv.Itoa(int((record.ResetTime-now)/1000)))
			c.Error(apperrors.NewRateLimitError("敏感操作过于频繁，请稍后再试"))
			c.Abort()
			return
		}

		c.Next()
	}
}
