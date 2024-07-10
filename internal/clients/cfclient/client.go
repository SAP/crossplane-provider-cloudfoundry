package cfclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	cfv3 "github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/config"

	cfhttp "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient/http"
)

// Client promotes the cfv3 client and adds an additional http executor for making direct HTTP requests against Cloud Foundry
type Client struct {
	*cfv3.Client
	httpExecutor *cfhttp.Executor
}

// New returns a new CF client
func New(config *config.Config) (*Client, error) {
	c := &Client{httpExecutor: cfhttp.NewExecutor(cfhttp.NewOAuthSessionManager(config), config.APIEndpointURL, config.UserAgent)}

	cf, err := cfv3.New(config)
	if err != nil {
		return nil, err
	}
	c.Client = cf
	return c, nil
}

// V3Client returns the underlying cfv3 client
func (c *Client) V3Client() *cfv3.Client {
	return c.Client
}

// HTTPDelete delete does an HTTP DELETE to the specified endpoint and returns the job ID if any
//
// This function takes the relative API resource path. If the resource returns an async job ID
// then the function returns the job GUID which the caller can reference via the job endpoint.
func (c *Client) HTTPDelete(ctx context.Context, path string) (string, error) {
	req := cfhttp.NewRequest(ctx, http.MethodDelete, path)
	// nolint: bodyclose
	resp, err := c.httpExecutor.ExecuteRequest(req)
	if err != nil {
		return "", fmt.Errorf("error deleting %s: %w", path, err)
	}
	defer func(b io.ReadCloser) {
		_ = b.Close()
	}(resp.Body)

	// some endpoints return accepted and others return no content
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return "", c.decodeError(resp)
	}
	return c.decodeJobIDOrBody(resp, nil)
}

// HTTPGet get does an HTTP GET to the specified endpoint and automatically handles unmarshalling
// the result JSON body
func (c *Client) HTTPGet(ctx context.Context, path string, result any) error {
	req := cfhttp.NewRequest(ctx, http.MethodGet, path)
	// nolint: bodyclose
	resp, err := c.httpExecutor.ExecuteRequest(req)
	if err != nil {
		return fmt.Errorf("error getting %s: %w", path, err)
	}
	defer func(b io.ReadCloser) {
		_ = b.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.decodeError(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, resp.Body)
		return fmt.Errorf("error decoding %s get response JSON before '%s': %w", path, buf.String(), err)
	}
	return nil
}

// HTTPPatch patch does an HTTP PATCH to the specified endpoint and automatically handles the result
// whether that's a JSON body or job ID.
//
// This function takes the relative API resource path, any parameters to PATCH and an optional
// struct to unmarshall the result body. If the resource returns an async job ID instead of a
// response body, then the body won't be unmarshalled and the function returns the job GUID
// which the caller can reference via the job endpoint.
func (c *Client) HTTPPatch(ctx context.Context, path string, params any, result any) (string, error) {

	req := cfhttp.NewRequest(ctx, http.MethodPatch, path).WithObject(params)
	// nolint: bodyclose
	resp, err := c.httpExecutor.ExecuteRequest(req)
	if err != nil {
		return "", fmt.Errorf("error updating %s: %w", path, err)
	}
	defer func(b io.ReadCloser) {
		_ = b.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return "", c.decodeError(resp)
	}
	return c.decodeJobIDOrBody(resp, result)
}

// HTTPPost post does an HTTP POST to the specified endpoint and automatically handles the result
// whether that's a JSON body or job ID.
//
// This function takes the relative API resource path, any parameters to POST and an optional
// struct to unmarshall the result body. If the resource returns an async job ID in the Location
// header then the job GUID is returned which the caller can reference via the job endpoint.
func (c *Client) HTTPPost(ctx context.Context, path string, params, result any) (string, error) {

	req := cfhttp.NewRequest(ctx, http.MethodPost, path).WithObject(params)
	// nolint: bodyclose
	resp, err := c.httpExecutor.ExecuteRequest(req)
	if err != nil {
		return "", fmt.Errorf("error creating %s: %w", path, err)
	}
	defer func(b io.ReadCloser) {
		_ = b.Close()
	}(resp.Body)

	// Endpoints return different status codes for posts
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", c.decodeError(resp)
	}
	return c.decodeJobIDOrBody(resp, result)
}

// decodeError attempts to unmarshall the response body as a CF error
func (c *Client) decodeError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cfclient: HTTP error (%d): %s", resp.StatusCode, resp.Status)
	}
	return fmt.Errorf("cfclient: HTTP error (%d): %s, %s", resp.StatusCode, resp.Status, body)
}

// decodeJobIDOrBody returns the jobGUID if specified in the Location response header or
// unmarshalls the JSON response body if no job ID and result is non nil
func (c *Client) decodeJobIDOrBody(resp *http.Response, result any) (string, error) {
	jobGUID := c.decodeJobID(resp)
	if jobGUID != "" {
		return jobGUID, nil
	}
	return "", c.decodeBody(resp, result)
}

// decodeJobID returns the jobGUID if specified in the Location response header
func (c *Client) decodeJobID(resp *http.Response) string {
	location, err := resp.Location()
	if err == nil && strings.Contains(location.Path, "jobs") {
		p := strings.Split(location.Path, "/")
		return p[len(p)-1]
	}
	return ""
}

// decodeBody unmarshalls the JSON response body if the result is non nil
func (c *Client) decodeBody(resp *http.Response, result any) error {
	if result != nil && resp.StatusCode != http.StatusNoContent {
		err := json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return fmt.Errorf("error decoding response JSON: %w", err)
		}
	}
	return nil
}
