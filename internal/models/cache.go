package models

import (
	"errors"
	"time"
)

var ErrCacheKeyDontExist = errors.New("cache key don't exist")

type Cache interface {
	Delete(key string) error
	Get(key string) (string, error)
	GetExpiry(key string, expiration time.Duration) (string, error)
	Put(key, value string) error
}
