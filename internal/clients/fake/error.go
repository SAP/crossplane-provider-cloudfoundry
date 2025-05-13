package fake

import "github.com/cloudfoundry/go-cfclient/v3/client"

// ErrNoResultReturned is error return by List()
var ErrNoResultReturned = client.ErrNoResultsReturned

// ErrExactlyOneResultNotReturned is error returned by Single()
var ErrExactlyOneResultNotReturned = client.ErrExactlyOneResultNotReturned
