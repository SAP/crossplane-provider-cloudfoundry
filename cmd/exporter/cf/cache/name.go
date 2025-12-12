package cache

import (
	"iter"
	"maps"
	"slices"
)

type CacheWithName[T ResourceWithName] interface {
	GetByName(name string) []T
	GetNames() []string
	StoreWithName(resources ...T)
	Len() int
	AllByNames() iter.Seq2[string, []T]
}

type cacheWithName[T ResourceWithName] struct {
	nameIndex map[string][]T
}

var _ CacheWithName[dummyResourceWithName] = &cacheWithName[dummyResourceWithName]{}

func NewWithName[T ResourceWithName]() CacheWithName[T] {
	return &cacheWithName[T]{
		nameIndex: make(map[string][]T),
	}
}

func (c *cacheWithName[T]) StoreWithName(resources ...T) {
	for _, resource := range resources {
		name := resource.GetName()
		c.nameIndex[name] = append(c.nameIndex[name], resource)
	}
}

func (c *cacheWithName[T]) GetByName(name string) []T {
	return c.nameIndex[name]
}

func (c *cacheWithName[T]) GetNames() []string {
	return slices.Sorted(maps.Keys(c.nameIndex))
}

func (c *cacheWithName[T]) Len() int {
	return len(c.nameIndex)
}

func (c *cacheWithName[T]) AllByNames() iter.Seq2[string, []T] {
	return maps.All(c.nameIndex)
}
