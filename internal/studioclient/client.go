package studioclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
)

const baseURL = "https://api.sanity.io/v2024-08-01/projects"

// Client is an HTTP client for the Sanity User Applications (Studio deploy) API.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Studio deploy client.
func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

// UserApplication represents a Sanity user application (deployed studio).
type UserApplication struct {
	ID       string `json:"id"`
	AppHost  string `json:"appHost"`
	Type     string `json:"type"`
	URLType  string `json:"urlType"`
	URL      string `json:"url,omitempty"`
	Title    string `json:"title,omitempty"`
}

// DeploymentResponse is the response from creating a deployment.
type DeploymentResponse struct {
	Location string `json:"location"`
}

func (c *Client) do(req *http.Request, v interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// FindApplication finds a user application by hostname.
func (c *Client) FindApplication(ctx context.Context, projectID, hostname string) (*UserApplication, error) {
	url := fmt.Sprintf("%s/%s/user-applications?appHost=%s&appType=studio", baseURL, projectID, hostname)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var apps []UserApplication
	if err := c.do(req, &apps); err != nil {
		return nil, err
	}

	for i := range apps {
		if strings.EqualFold(apps[i].AppHost, hostname) {
			return &apps[i], nil
		}
	}
	return nil, fmt.Errorf("application with hostname %q not found", hostname)
}

// GetApplication gets a user application by ID.
func (c *Client) GetApplication(ctx context.Context, projectID, appID string) (*UserApplication, error) {
	url := fmt.Sprintf("%s/%s/user-applications/%s", baseURL, projectID, appID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var app UserApplication
	if err := c.do(req, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// CreateApplication creates a new user application for studio hosting.
func (c *Client) CreateApplication(ctx context.Context, projectID, hostname string) (*UserApplication, error) {
	url := fmt.Sprintf("%s/%s/user-applications?appType=studio", baseURL, projectID)

	payload := fmt.Sprintf(`{"appHost":%q,"type":"studio","urlType":"internal"}`, hostname)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var app UserApplication
	if err := c.do(req, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// DeleteApplication deletes a user application.
func (c *Client) DeleteApplication(ctx context.Context, projectID, appID string) error {
	url := fmt.Sprintf("%s/%s/user-applications/%s", baseURL, projectID, appID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// DeployOptions are options for creating a deployment.
type DeployOptions struct {
	BundlePath   string // path to the .tar.gz file
	AutoUpdates  bool
	Version      string // sanity package version, e.g. "3.0.0"
}

// CreateDeployment uploads a tarball and creates a new deployment.
func (c *Client) CreateDeployment(ctx context.Context, projectID, appID string, opts *DeployOptions) (*DeploymentResponse, error) {
	url := fmt.Sprintf("%s/%s/user-applications/%s/deployments?appType=studio", baseURL, projectID, appID)

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write the multipart form in a goroutine to stream it
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer writer.Close()

		// isAutoUpdating field
		autoUpdating := "false"
		if opts.AutoUpdates {
			autoUpdating = "true"
		}
		if err := writer.WriteField("isAutoUpdating", autoUpdating); err != nil {
			errCh <- fmt.Errorf("writing isAutoUpdating field: %w", err)
			return
		}

		// version field
		if opts.Version != "" {
			if err := writer.WriteField("version", opts.Version); err != nil {
				errCh <- fmt.Errorf("writing version field: %w", err)
				return
			}
		}

		// tarball file field
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="tarball"; filename="app.tar.gz"`)
		h.Set("Content-Type", "application/gzip")
		part, err := writer.CreatePart(h)
		if err != nil {
			errCh <- fmt.Errorf("creating tarball part: %w", err)
			return
		}

		f, err := os.Open(opts.BundlePath)
		if err != nil {
			errCh <- fmt.Errorf("opening bundle file: %w", err)
			return
		}
		defer f.Close()

		if _, err := io.Copy(part, f); err != nil {
			errCh <- fmt.Errorf("writing bundle data: %w", err)
			return
		}

		errCh <- nil
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var resp DeploymentResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	// Check if the multipart writer encountered an error
	if writeErr := <-errCh; writeErr != nil {
		return nil, writeErr
	}

	return &resp, nil
}
