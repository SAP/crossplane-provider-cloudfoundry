package clients

import (
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

// ErrorIsNotFound return true if error is not nil and is a not found issue.
func ErrorIsNotFound(err error) bool {
	if err == nil {
		return false
	}

	if err.Error() == client.ErrNoResultsReturned.Error() || // first()
		err.Error() == client.ErrExactlyOneResultNotReturned.Error() { // single()
		return true
	}

	return strings.Contains(err.Error(), "NotFound")
}

// IgnoreNotFoundErr returns nil if the error a not found issue.
func IgnoreNotFoundErr(err error) error {
	if err == nil {
		return nil
	}
	switch err.Error() {
	case client.ErrNoResultsReturned.Error():
		return nil
	case client.ErrExactlyOneResultNotReturned.Error():
		return nil
	default:
		if strings.Contains(err.Error(), "NotFound") {
			return nil
		}
		return err
	}
}
