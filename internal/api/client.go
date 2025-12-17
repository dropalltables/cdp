package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client is the Coolify API client
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// APIError represents an error from the Coolify API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

// IsConflict returns true if the error is a 409 Conflict
func IsConflict(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 409
	}
	return false
}

// IsNotFound returns true if the error is a 404 Not Found
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}

// NewClient creates a new Coolify API client
func NewClient(baseURL, token string) *Client {
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	// Ensure it has /api/v1 suffix
	if !strings.HasSuffix(baseURL, "/api/v1") {
		baseURL = baseURL + "/api/v1"
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// request performs an HTTP request
func (c *Client) request(method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	debug := os.Getenv("CDP_DEBUG") != ""

	if debug {
		fmt.Printf("[API] %s %s\n", method, reqURL)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if debug {
		// Truncate for readability
		body := string(respBody)
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		fmt.Printf("[API] Response %d: %s\n", resp.StatusCode, body)
	}

	if resp.StatusCode >= 400 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request
func (c *Client) Get(path string, result interface{}) error {
	return c.request(http.MethodGet, path, nil, result)
}

// Post performs a POST request
func (c *Client) Post(path string, body interface{}, result interface{}) error {
	return c.request(http.MethodPost, path, body, result)
}

// Patch performs a PATCH request
func (c *Client) Patch(path string, body interface{}, result interface{}) error {
	return c.request(http.MethodPatch, path, body, result)
}

// Delete performs a DELETE request
func (c *Client) Delete(path string) error {
	return c.request(http.MethodDelete, path, nil, nil)
}

// GetWithParams performs a GET request with query parameters
func (c *Client) GetWithParams(path string, params map[string]string, result interface{}) error {
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		path = path + "?" + values.Encode()
	}
	return c.Get(path, result)
}
