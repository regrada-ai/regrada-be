package regrada

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const DefaultAPIURL = "https://api.regrada.com"

// Client is the HTTP client for Regrada API
type Client struct {
	baseURL   string
	apiKey    string
	projectID string
	client    *http.Client
}

// NewClient creates a new Regrada API client
func NewClient(apiKey, projectID string) *Client {
	return &Client{
		baseURL:   DefaultAPIURL,
		apiKey:    apiKey,
		projectID: projectID,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadTraces uploads a batch of traces
func (c *Client) UploadTraces(ctx context.Context, traces []Trace) error {
	url := fmt.Sprintf("%s/v1/projects/%s/traces/batch", c.baseURL, c.projectID)
	
	payload := map[string]interface{}{
		"traces": traces,
	}
	
	return c.post(ctx, url, payload, nil)
}

// UploadTestRun uploads a test run result
func (c *Client) UploadTestRun(ctx context.Context, run TestRun) error {
	url := fmt.Sprintf("%s/v1/projects/%s/test-runs", c.baseURL, c.projectID)
	return c.post(ctx, url, run, nil)
}

// GetProject retrieves project information
func (c *Client) GetProject(ctx context.Context) (*Project, error) {
	url := fmt.Sprintf("%s/v1/projects/%s", c.baseURL, c.projectID)
	var project Project
	err := c.get(ctx, url, &project)
	return &project, err
}

// Helper methods
func (c *Client) post(ctx context.Context, url string, payload interface{}, result interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		var apiErr APIError
		json.NewDecoder(resp.Body).Decode(&apiErr)
		return &apiErr
	}
	
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	
	return nil
}

func (c *Client) get(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		var apiErr APIError
		json.NewDecoder(resp.Body).Decode(&apiErr)
		return &apiErr
	}
	
	return json.NewDecoder(resp.Body).Decode(result)
}

// APIError represents an API error response
type APIError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Project represents a project
type Project struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
}
