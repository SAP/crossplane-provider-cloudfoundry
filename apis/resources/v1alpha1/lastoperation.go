package v1alpha1

const (
	// LastOperationCreate for create
	LastOperationCreate = "create"

	// LastOperationUpdate for update
	LastOperationUpdate = "update"

	// LastOperationDelete for delete
	LastOperationDelete = "delete"

	// LastOperationInitial signals that the last operation type is initialized
	LastOperationInitial = "initial"

	// LastOperationInProgress signals that the last operation type is in progress
	LastOperationInProgress = "in progress"

	// LastOperationSucceeded signals that the last operation type has succeeded
	LastOperationSucceeded = "succeeded"

	// LastOperationFailed signals that the last operation type has failed
	LastOperationFailed = "failed"
)

// LastOperation records the last performed operation type and state on async resource .
type LastOperation struct {
	// (String) the type of last operation perform on the resource.
	Type string `json:"type,omitempty" tf:"type,omitempty"`

	// (String) The state of the last operation
	State string `json:"state,omitempty" tf:"state,omitempty"`

	// (String) A description of the last operation
	Description string `json:"description,omitempty" tf:"description,omitempty"`

	// (String) The date and time when the resource was created in RFC3339 format.
	CreatedAt string `json:"createdAt,omitempty" tf:"created_at,omitempty"`

	// (String) The date and time when the resource was updated in RFC3339 format.
	UpdatedAt string `json:"updatedAt,omitempty" tf:"updated_at,omitempty"`
}
