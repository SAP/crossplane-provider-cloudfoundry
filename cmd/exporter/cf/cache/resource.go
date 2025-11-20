package cache

import (
	"bytes"
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/yaml"

	"k8s.io/utils/ptr"
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

type ResourceWithComment struct {
	commentString *string
}

func (r *ResourceWithComment) Comment() (string, bool) {
	if r.commentString == nil {
		return "", false
	}
	return *r.commentString, true
}

func (r *ResourceWithComment) SetComment(comment string) {
	r.commentString = nil
	r.AddComment(comment)
}

func (r *ResourceWithComment) AddComment(comment string) {
	c := ""
	if r.commentString != nil {
		c = *r.commentString
	}
	sBuffer := bytes.NewBufferString(c)
	fmt.Fprintln(sBuffer, comment)
	r.commentString = ptr.To(sBuffer.String())
}

func (r *ResourceWithComment) CloneComment(other *ResourceWithComment) {
	c, ok := other.Comment()
	if ok {
		r.commentString = &c
	} else {
		r.commentString = nil
	}
}

type ResourceWithGUIDAndName interface {
	ResourceWithGuid
	ResourceWithName
	yaml.CommentedYAML
}

type dummyResourceWithGUIDAndName struct {
	dummyResourceWithGuid
	dummyResourceWithName
	*ResourceWithComment
}

var _ ResourceWithGUIDAndName = dummyResourceWithGUIDAndName{}
