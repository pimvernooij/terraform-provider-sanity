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

// SchemaEntry is a single schema entry in a PUT request.
type SchemaEntry struct {
	Workspace SchemaWorkspace `json:"workspace"`
	Schema    json.RawMessage `json:"schema"`
	Version   string          `json:"version"`
	Tag       string          `json:"tag,omitempty"`
}

// putSchemasBody is the top-level PUT request body.
type putSchemasBody struct {
	Schemas []SchemaEntry `json:"schemas"`
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

	// The API may return either a single object or an array.
	// Try single object first, fall back to array.
	var raw json.RawMessage
	if err := c.do(req, &raw); err != nil {
		return nil, err
	}

	// Try to decode as single document
	var doc SchemaDocument
	if err := json.Unmarshal(raw, &doc); err == nil && doc.ID != "" {
		return &doc, nil
	}

	// Fall back to array
	var docs []SchemaDocument
	if err := json.Unmarshal(raw, &docs); err != nil {
		return nil, fmt.Errorf("decoding schema response: %w", err)
	}
	for i := range docs {
		if docs[i].ID == schemaID {
			return &docs[i], nil
		}
	}
	if len(docs) > 0 {
		return &docs[0], nil
	}
	return nil, fmt.Errorf("schema %s not found", schemaID)
}

// PutSchemas upserts a schema for a project/dataset. Returns the resulting schema document.
// The API expects: {"schemas": [{workspace, schema, version, tag}]}
func (c *Client) PutSchemas(ctx context.Context, projectID, dataset string, entry *SchemaEntry) (*SchemaDocument, error) {
	payload := putSchemasBody{
		Schemas: []SchemaEntry{*entry},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.schemasURL(projectID, dataset), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var doc SchemaDocument
	if err := c.do(req, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
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
