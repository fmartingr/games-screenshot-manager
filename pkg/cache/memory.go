package cache

import (
	"sync"
	"time"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/sirupsen/logrus"
)

type MemoryCache struct {
	logger *logrus.Entry
	data   map[string]string
	dataMu sync.RWMutex
}

func (c *MemoryCache) Get(key string) (result string, err error) {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()

	result, exists := c.data[key]
	if !exists {
		return result, models.ErrCacheKeyDontExist
	}

	return
}

func (c *MemoryCache) Put(key, value string) error {
	c.dataMu.Lock()
	c.data[key] = value
	c.dataMu.Unlock()

	return nil
}

func (c *MemoryCache) GetExpiry(key string, expiration time.Duration) (string, error) {
	// Since this is a in-memory storage, expiration is not required as of now.
	return c.Get(key)
}

func (c *MemoryCache) Delete(key string) error {
	c.dataMu.Lock()
	delete(c.data, key)
	c.dataMu.Unlock()
	return nil
}

func NewMemoryCache(logger *logrus.Logger) *MemoryCache {
	return &MemoryCache{
		logger: logger.WithField("from", "cache.file"),
		data:   make(map[string]string),
	}
}
