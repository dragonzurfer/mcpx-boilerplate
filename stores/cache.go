package stores

import (
	"container/list"
	"sync"
	"time"
)

type CacheConfig struct {
	TTL        time.Duration
	MaxEntries int
}

type CacheSetInput struct {
	Key   string
	Value any
}

type Cache struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	entries    map[string]*cacheEntry
	order      *list.List
}

type cacheEntry struct {
	key       string
	value     any
	expiresAt time.Time
	element   *list.Element
}

func NewCache(config CacheConfig) *Cache {
	if !cacheEnabled(config) {
		return nil
	}
	return &Cache{
		ttl:        config.TTL,
		maxEntries: config.MaxEntries,
		entries:    map[string]*cacheEntry{},
		order:      list.New(),
	}
}

func (c *Cache) Get(key string) (any, bool) {
	if c == nil || key == "" {
		return nil, false
	}

	now := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.isExpired(entry, now) {
		c.removeEntry(entry)
		return nil, false
	}

	c.order.MoveToFront(entry.element)
	return entry.value, true
}

func (c *Cache) Set(input CacheSetInput) {
	if c == nil || input.Key == "" {
		return
	}

	now := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.setLocked(input, now)
}

func (c *Cache) setLocked(input CacheSetInput, now time.Time) {
	entry, ok := c.entries[input.Key]
	if ok {
		entry.value = input.Value
		entry.expiresAt = now.Add(c.ttl)
		c.order.MoveToFront(entry.element)
		return
	}

	element := c.order.PushFront(input.Key)
	entry = &cacheEntry{
		key:       input.Key,
		value:     input.Value,
		expiresAt: now.Add(c.ttl),
		element:   element,
	}
	c.entries[input.Key] = entry

	c.evictLocked()
}

func (c *Cache) evictLocked() {
	for len(c.entries) > c.maxEntries {
		element := c.order.Back()
		if element == nil {
			return
		}
		key, ok := element.Value.(string)
		if !ok {
			c.order.Remove(element)
			continue
		}
		entry := c.entries[key]
		if entry == nil {
			c.order.Remove(element)
			continue
		}
		c.removeEntry(entry)
	}
}

func (c *Cache) removeEntry(entry *cacheEntry) {
	if entry == nil {
		return
	}
	delete(c.entries, entry.key)
	c.order.Remove(entry.element)
}

func (c *Cache) isExpired(entry *cacheEntry, now time.Time) bool {
	if entry == nil {
		return true
	}
	return now.After(entry.expiresAt)
}

func cacheEnabled(config CacheConfig) bool {
	if config.TTL <= 0 {
		return false
	}
	if config.MaxEntries <= 0 {
		return false
	}
	return true
}
