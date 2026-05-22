package cache

import (
	"sync"
	"time"
)

type Item struct {
	Value     []byte
	ExpiresAt time.Time
}

type Memory struct {
	mu    sync.RWMutex
	items map[string]Item
}

func NewMemory() *Memory {
	return &Memory{items: map[string]Item{}}
}
