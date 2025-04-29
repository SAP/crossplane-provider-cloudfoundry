package v1alpha1

type Operation struct {
	ID    *string `json:"id,omitempty"`
	Error *string `json:"error,omitempty"`
	State *string `json:"state,omitempty"`
}

func (o *Operation) IsRunning() bool {
	if o.State == nil {
		return true
	}
	return *o.State == "RUNNING"
}

func (o *Operation) HasError() bool {
	if o.State == nil {
		return false
	}
	return *o.State == "ABORTED"
}

func (o *Operation) GetError() string {
	if !o.HasError() {
		return ""
	}
	if len(*o.Error) > 0 {
		return *o.Error
	}
	return *o.State
}
