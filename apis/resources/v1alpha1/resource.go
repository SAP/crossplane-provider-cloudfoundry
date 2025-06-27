package v1alpha1

// Resource represents a Cloud Foundry resource.
type Resource struct {
	// (String) The GUID of the Cloud Foundry resource.
	GUID string `json:"guid,omitempty"`

	// (String) The date and time when the resource was created in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	CreatedAt *string `json:"createdAt,omitempty"`

	// (String) The date and time when the resource was updated in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// ResourceMetadata represents the metadata associated with a Cloud Foundry resource.
type ResourceMetadata struct {
	// (Map of String) The annotations associated with the resource. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	Annotations map[string]*string `json:"annotations,omitempty"`

	// (Map of String) The labels associated with the resource. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	Labels map[string]*string `json:"labels,omitempty"`
}
