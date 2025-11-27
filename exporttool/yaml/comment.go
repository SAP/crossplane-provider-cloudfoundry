package yaml

import (
	"bytes"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/utils/ptr"
)

// CommentedYAML is an interface for resources that can have associated comments
// in their YAML representation. When a resource implements this interface, the
// output YAML will be commented out if the Comment method returns true as the
// second result. The first (string) result will be placed above the YAML as a comment.
type CommentedYAML interface {
	// Comment returns the comment text and a boolean indicating whether the resource
	// should be commented out in the YAML output.
	Comment() (string, bool)
}

// ResourceWithComment wraps a resource of any type with an optional comment.
// This allows any type to implement the CommentedYAML interface.
type ResourceWithComment struct {
	// comment holds the optional comment text for the resource.
	comment *string
	// Object is the embedded resource being wrapped.
	resource.Object
}

// Ensure ResourceWithComment implements CommentedYAML interface.
var _ CommentedYAML = &ResourceWithComment{}

// NewResourceWithComment creates a new ResourceWithComment wrapping the given resource
// with no initial comment.
func NewResourceWithComment(res resource.Object) *ResourceWithComment {
	return &ResourceWithComment{
		comment: nil,
		Object:  res,
	}
}

// Comment returns the comment text and whether a comment is set.
// Returns an empty string and false if no comment is set.
func (r *ResourceWithComment) Comment() (string, bool) {
	if r.comment == nil {
		return "", false
	}
	return *r.comment, true
}

// Resource returns the underlying wrapped resource object.
func (r *ResourceWithComment) Resource() resource.Object {
	return r.Object
}

// SetResource replaces the underlying wrapped resource object with the provided resource.
func (r *ResourceWithComment) SetResource(res resource.Object) {
	r.Object = res
}

// SetComment replaces any existing comment with the provided comment text.
// This clears the previous comment and sets a new one.
func (r *ResourceWithComment) SetComment(comment string) {
	r.comment = nil
	r.AddComment(comment)
}

// AddComment appends the provided comment text to any existing comment.
// If no comment exists, this sets the initial comment. If a comment already exists,
// the new text is appended on a new line.
func (r *ResourceWithComment) AddComment(comment string) {
	c := ""
	if r.comment != nil {
		c = *r.comment
	}
	sBuffer := bytes.NewBufferString(c)
	fmt.Fprintln(sBuffer, comment)
	r.comment = ptr.To(sBuffer.String())
}

// CloneComment copies the comment from another CommentedYAML resource to this one.
// If the source resource has a comment, it is copied. Otherwise, any existing comment
// on this resource is cleared.
func (r *ResourceWithComment) CloneComment(other CommentedYAML) {
	c, ok := other.Comment()
	if ok {
		r.comment = &c
	} else {
		r.comment = nil
	}
}
