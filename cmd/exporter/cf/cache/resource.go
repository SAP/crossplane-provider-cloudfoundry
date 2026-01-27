package cache

import (
	"github.com/SAP/xp-clifford/yaml"
)

type ResourceWithGUID interface {
	GetGUID() string
}

type dummyResourceWithGUID struct{}

var _ ResourceWithGUID = dummyResourceWithGUID{}

func (r dummyResourceWithGUID) GetGUID() string {
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
	ResourceWithGUID
	ResourceWithName
	yaml.CommentedYAML
}

type dummyResourceWithGUIDAndName struct {
	dummyResourceWithGUID
	dummyResourceWithName
	*yaml.ResourceWithComment
}

var _ ResourceWithGUIDAndName = dummyResourceWithGUIDAndName{}
