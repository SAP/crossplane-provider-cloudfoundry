package cfclient

import (
	"strings"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
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
