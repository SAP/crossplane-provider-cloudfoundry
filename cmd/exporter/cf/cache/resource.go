package cache

type Convertable[F, T any] interface {
	Convert(resource T) F
}

type ResourceWithGuid interface {
	GetGUID() string
}

type dummyResourceWithGuid struct{}

var _ ResourceWithGuid = dummyResourceWithGuid{}

func (r dummyResourceWithGuid) GetGUID() string {
	return "dummyGUID"
}

type ResourceWithName interface {
	GetName() string
}

type dummyResourceWithName struct{}

var _ ResourceWithName = dummyResourceWithName{}

func (r dummyResourceWithName) GetName() string {
	return "dummyName"
}

type ResourceWithGUIDAndName interface {
	ResourceWithGuid
	ResourceWithName
}

type dummyResourceWithGUIDAndName struct {
	dummyResourceWithGuid
	dummyResourceWithName
}

var _ ResourceWithGUIDAndName = dummyResourceWithGUIDAndName{}
