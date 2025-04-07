package v1alpha1

// Resource is a generic struct that represents a Cloud Foundry resource.
type Resource struct {
	// The GUID of the Cloud Foundry resource
	GUID string `json:"guid,omitempty"`

	// The date and time when the resource was created in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	CreatedAt *string `json:"createdAt,omitempty"`

	// The date and time when the resource was updated in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// ResourceMetadata is a struct that represents the metadata associated with Cloud Foundry resources.
type ResourceMetadata struct {
	// The annotations associated with Cloud Foundry resources. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	Annotations map[string]*string `json:"annotations,omitempty"`

	// The labels associated with Cloud Foundry resources. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	Labels map[string]*string `json:"labels,omitempty"`
}
