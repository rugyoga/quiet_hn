package hn

import (
  "sync"
)

type ItemCache struct {
	cache map[int]*Item
	sync.RWMutex
}

var internal ItemCache

func ItemCacheSingleton() *ItemCache {
  internal.Lock()
  defer internal.Unlock()
  if (internal.cache == nil) {
    internal.cache = make(map[int]*Item)
  }
  return &internal
}

func (c *ItemCache) Put(id int, item *Item) {
  c.Lock()
  defer c.Unlock()
  c.cache[id] = item
}

func (c *ItemCache) Get(id int) (*Item, bool) {
  c.RLock()
  defer c.RUnlock()
  item, ok := c.cache[id]
  return item, ok
}
