package cache

import (
	"iter"
	"maps"
	"slices"
)

type CacheWithGUID[T ResourceWithGuid] interface {
	GetByGUID(guid string) T
	GetGUIDs() []string
	Len() int
	StoreWithGUID(resources ...T)
	AllByGUIDs() iter.Seq2[string, T]
}

type cacheWithGUID[T ResourceWithGuid] struct {
	guidIndex map[string]T
}

var _ CacheWithGUID[dummyResourceWithGuid] = &cacheWithGUID[dummyResourceWithGuid]{}

func NewWithGUID[T ResourceWithGuid]() CacheWithGUID[T] {
	return &cacheWithGUID[T]{
		guidIndex: make(map[string]T),
	}
}

func (c *cacheWithGUID[T]) StoreWithGUID(resources ...T) {
	for _, resource := range resources {
		c.guidIndex[resource.GetGUID()] = resource
	}
}

func (c *cacheWithGUID[T]) GetByGUID(guid string) T {
	return c.guidIndex[guid]
}

func (c *cacheWithGUID[T]) GetGUIDs() []string {
	return slices.Sorted(maps.Keys(c.guidIndex))
}

func (c *cacheWithGUID[T]) Len() int {
	return len(c.guidIndex)
}

func (c *cacheWithGUID[T]) AllByGUIDs() iter.Seq2[string, T] {
	return maps.All(c.guidIndex)
}
