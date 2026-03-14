package schemaclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.sanity.io/v2025-03-01/projects"

// Client is an HTTP client for the Sanity Schema API.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Schema API client using the provided HTTP client
// (which should already be configured with authentication).
func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

// SchemaWorkspace represents the workspace portion of a schema document.
type SchemaWorkspace struct {
	Name  string `json:"name"`
	Title string `json:"title,omitempty"`
}

// SchemaDocument represents a single schema document returned by the API.
type SchemaDocument struct {
	ID        string          `json:"_id"`
	Type      string          `json:"_type"`
	Version   string          `json:"version"`
	Tag       string          `json:"tag,omitempty"`
	Workspace SchemaWorkspace `json:"workspace"`
	Schema    json.RawMessage `json:"schema"`
}

// PutSchemasRequest is the request body for upserting schemas.
type PutSchemasRequest struct {
	Workspace SchemaWorkspace `json:"workspace"`
	Schema    json.RawMessage `json:"schema"`
	Version   string          `json:"version"`
	Tag       string          `json:"tag,omitempty"`
}

func (c *Client) schemasURL(projectID, dataset string) string {
	return fmt.Sprintf("%s/%s/datasets/%s/schemas", baseURL, projectID, dataset)
}

func (c *Client) schemaURL(projectID, dataset, schemaID string) string {
	return fmt.Sprintf("%s/%s/datasets/%s/schemas/%s", baseURL, projectID, dataset, schemaID)
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

// ListSchemas returns all schema documents for a project/dataset.
func (c *Client) ListSchemas(ctx context.Context, projectID, dataset string) ([]SchemaDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.schemasURL(projectID, dataset), nil)
	if err != nil {
		return nil, err
	}

	var docs []SchemaDocument
	if err := c.do(req, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// GetSchema returns a single schema document by ID.
func (c *Client) GetSchema(ctx context.Context, projectID, dataset, schemaID string) (*SchemaDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.schemaURL(projectID, dataset, schemaID), nil)
	if err != nil {
		return nil, err
	}

	var doc SchemaDocument
	if err := c.do(req, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// PutSchemas upserts schemas for a project/dataset. Returns the resulting schema documents.
func (c *Client) PutSchemas(ctx context.Context, projectID, dataset string, putReq *PutSchemasRequest) ([]SchemaDocument, error) {
	// The API expects an array of schema entries
	payload := []PutSchemasRequest{*putReq}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.schemasURL(projectID, dataset), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var docs []SchemaDocument
	if err := c.do(req, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// DeleteSchema deletes a schema document by ID.
func (c *Client) DeleteSchema(ctx context.Context, projectID, dataset, schemaID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.schemaURL(projectID, dataset, schemaID), nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

// SchemaID computes the schema document ID from workspace name and optional tag.
func SchemaID(workspaceName, tag string) string {
	if tag != "" {
		return fmt.Sprintf("_.schemas.%s.%s", workspaceName, tag)
	}
	return fmt.Sprintf("_.schemas.%s", workspaceName)
}
