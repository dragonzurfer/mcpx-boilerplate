package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	count    int
	resetsAt time.Time
}

// RateLimiter is a lightweight per-user/IP limiter to avoid external deps.
func RateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := map[string]*rateBucket{}

	return func(c *gin.Context) {
		key := c.ClientIP()
		if user, ok := CurrentUser(c); ok {
			key = fmt.Sprintf("user-%d", user.ID)
		}

		now := time.Now()
		mu.Lock()
		b, ok := buckets[key]
		if !ok || now.After(b.resetsAt) {
			b = &rateBucket{count: 0, resetsAt: now.Add(window)}
			buckets[key] = b
		}

		if b.count >= maxRequests {
			retry := int(time.Until(b.resetsAt).Seconds())
			if retry < 1 {
				retry = 1
			}
			mu.Unlock()
			c.Header("Retry-After", fmt.Sprintf("%d", retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		b.count++
		mu.Unlock()
		c.Next()
	}
}

// GlobalRateLimiter caps all requests that pass through this middleware using a single shared bucket.
func GlobalRateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	bucket := rateBucket{
		count:    0,
		resetsAt: time.Now(),
	}

	return func(c *gin.Context) {
		now := time.Now()
		mu.Lock()
		if bucket.resetsAt.IsZero() || now.After(bucket.resetsAt) {
			bucket.count = 0
			bucket.resetsAt = now.Add(window)
		}

		if bucket.count >= maxRequests {
			retry := int(time.Until(bucket.resetsAt).Seconds())
			if retry < 1 {
				retry = 1
			}
			mu.Unlock()
			c.Header("Retry-After", fmt.Sprintf("%d", retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		bucket.count++
		mu.Unlock()
		c.Next()
	}
}
