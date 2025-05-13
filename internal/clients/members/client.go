package members

import (
	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
)

// Client promotes the cfv3 client
type Client struct {
	*cfv3.Client
}

// NewClient returns a new CF client
func NewClient(config *config.Config) (*Client, error) {

	cf, err := cfv3.New(config)
	if err != nil {
		return nil, err
	}

	return &Client{Client: cf}, nil
}

// V3Client returns the underlying cfv3 client
func (c *Client) V3Client() *cfv3.Client {
	return c.Client
}
