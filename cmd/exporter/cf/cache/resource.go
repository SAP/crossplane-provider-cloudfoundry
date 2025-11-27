package cache

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"
)

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
	yaml.CommentedYAML
}

type dummyResourceWithGUIDAndName struct {
	dummyResourceWithGuid
	dummyResourceWithName
	*yaml.ResourceWithComment
}

var _ ResourceWithGUIDAndName = dummyResourceWithGUIDAndName{}
