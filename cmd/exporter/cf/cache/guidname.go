package cache

type CacheWithGUIDAndName[T ResourceWithGUIDAndName] interface {
	CacheWithGUID[T]
	CacheWithName[T]
	StoreWithGUIDAndName(resources ...T)
}

type cacheWithGUIDAndName[T ResourceWithGUIDAndName] struct {
	CacheWithGUID[T]
	CacheWithName[T]
}

var _ CacheWithGUIDAndName[dummyResourceWithGUIDAndName] = &cacheWithGUIDAndName[dummyResourceWithGUIDAndName]{}

func NewWithGUIDAndName[T ResourceWithGUIDAndName]() CacheWithGUIDAndName[T] {
	return &cacheWithGUIDAndName[T]{
		CacheWithGUID: NewWithGUID[T](),
		CacheWithName: NewWithName[T](),
	}
}

func (c *cacheWithGUIDAndName[T]) Len() int {
	return c.CacheWithGUID.Len()
}

func (c *cacheWithGUIDAndName[T]) StoreWithGUIDAndName(resources ...T) {
	c.CacheWithGUID.StoreWithGUID(resources...)
	c.CacheWithName.StoreWithName(resources...)
}

func (c *cacheWithGUIDAndName[T]) StoreWithName(_ ...T) {
	panic("StoreWithName shall not be used here")
}

func (c *cacheWithGUIDAndName[T]) StoreWithGUID(_ ...T) {
	panic("StoreWithGUID shall not be used here")
}
