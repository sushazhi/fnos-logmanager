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
	// oldestKey tracks the oldest entry for deterministic LRU eviction.
	oldestKey  string
	oldestTime int64
}

func newRateLimitStore() *rateLimitStore {
	return &rateLimitStore{
		store: make(map[string]*types.RateLimitRecord),
	}
}

// bump atomically increments the counter for key, resetting it when the window
// has elapsed, and returns a snapshot of the resulting record. The whole
// read-modify-write happens under a single write lock, which eliminates the
// data race that occurred when middlewares incremented a shared *Record that
// had been returned by an earlier read-locked lookup.
func (rls *rateLimitStore) bump(key string, now, windowMs int64) *types.RateLimitRecord {
	rls.mu.Lock()
	defer rls.mu.Unlock()

	rec := rls.store[key]
	if rec == nil {
		rec = &types.RateLimitRecord{
			Count:     0,
			ResetTime: now + windowMs,
		}
	}

	if now > rec.ResetTime {
		rec.Count = 1
		rec.ResetTime = now + windowMs
	} else {
		rec.Count++
	}

	if len(rls.store) >= maxRateLimitEntries {
		// Evict the oldest entry deterministically instead of random.
		// If the tracked oldest key is stale, scan for the real oldest.
		if rls.oldestKey == "" || rls.store[rls.oldestKey] == nil {
			var minTime int64 = 1<<63 - 1
			var minKey string
			for k, v := range rls.store {
				if v.ResetTime < minTime {
					minTime = v.ResetTime
					minKey = k
				}
			}
			rls.oldestKey = minKey
			rls.oldestTime = minTime
		}
		if rls.oldestKey != "" {
			delete(rls.store, rls.oldestKey)
			rls.oldestKey = ""
		}
	}
	// Track the new oldest entry
	if rls.oldestKey == "" || rec.ResetTime < rls.oldestTime {
		rls.oldestTime = rec.ResetTime
	}

	rls.store[key] = rec
	cp := *rec
	return &cp
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
	globalRateLimit    = newRateLimitStore()
	apiRateLimitStore  = newRateLimitStore()
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

	record := globalRateLimit.bump(ip, now, cfg.RateLimit.WindowMs)

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

		record := apiRateLimitStore.bump(ip, now, windowMs)

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

		record := sensitiveRateLimit.bump(key, now, windowMs)

		if record.Count > maxRequests {
			c.Header("Retry-After", strconv.Itoa(int((record.ResetTime-now)/1000)))
			c.Error(apperrors.NewRateLimitError("敏感操作过于频繁，请稍后再试"))
			c.Abort()
			return
		}

		c.Next()
	}
}
