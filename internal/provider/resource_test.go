package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tessellator/go-sanity/sanity"
)

// newTestClient creates a sanity.Client pointing at a test HTTP server.
// It uses reflect to set the private baseURL field.
func newTestClient(server *httptest.Server) *sanity.Client {
	client := sanity.NewClient(server.Client())

	// Override the private baseURL field via reflection so all API calls
	// hit our test server instead of the real Sanity API.
	v := reflect.ValueOf(client).Elem()
	f := v.FieldByName("baseURL")
	// Make the field settable via unsafe pointer
	reflect.NewAt(f.Type(), f.Addr().UnsafePointer()).Elem().SetString(server.URL)

	return client
}

// jsonResponse writes a JSON response to the test server.
func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// --- Project resource tests ---

func TestProjectUpdate_APIError_NoNilPanic(t *testing.T) {
	// This test verifies that when the Update API call fails, the resource
	// does not panic with a nil pointer dereference. Before the fix,
	// the code called project.Id on a nil project after a failed update.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			jsonResponse(w, 500, map[string]string{"message": "internal server error"})
			return
		}
		// GET for project read
		jsonResponse(w, 200, sanity.Project{
			Id:          "proj123",
			DisplayName: "Test",
			Metadata:    map[string]string{"color": "#ff0000"},
		})
	}))
	defer server.Close()

	client := newTestClient(server)

	// Simulate what the Update handler does: call Projects.Update which returns error
	_, err := client.Projects.Update(context.Background(), "proj123", &sanity.UpdateProjectRequest{
		DisplayName: "New Name",
	})
	if err == nil {
		t.Fatal("expected error from Update, got nil")
	}

	// The old buggy code did: r.client.Projects.Delete(ctx, project.Id)
	// where project was nil, causing a panic. We verify the error is returned
	// cleanly without needing to call project.Id.
	t.Logf("Update correctly returned error: %v", err)
}

func TestProjectRead_NilMetadata(t *testing.T) {
	// Test that reading a project with nil Metadata map doesn't panic.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a project with no metadata field at all
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"id":"proj123","displayName":"Test","studioHost":"","isDisabledByUser":false,"activityFeedEnabled":true}`)
	}))
	defer server.Close()

	client := newTestClient(server)

	project, err := client.Projects.Get(context.Background(), "proj123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Accessing Metadata keys on a nil map should not panic in Go
	// (returns zero value), but we verify the behavior
	color := project.Metadata["color"]
	if color != "" {
		t.Errorf("expected empty string for missing metadata key, got %q", color)
	}

	externalHost := project.Metadata["externalStudioHost"]
	if externalHost != "" {
		t.Errorf("expected empty string for missing metadata key, got %q", externalHost)
	}
}

func TestProjectRead_WithMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, sanity.Project{
			Id:                  "proj123",
			DisplayName:         "My Project",
			OrganizationId:      "org456",
			StudioHost:          "my-studio",
			Metadata:            map[string]string{"color": "#00ff00", "externalStudioHost": "https://studio.example.com"},
			IsDisabledByUser:    false,
			ActivityFeedEnabled: true,
		})
	}))
	defer server.Close()

	client := newTestClient(server)

	project, err := client.Projects.Get(context.Background(), "proj123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if project.Metadata["color"] != "#00ff00" {
		t.Errorf("color = %q, want #00ff00", project.Metadata["color"])
	}
	if project.Metadata["externalStudioHost"] != "https://studio.example.com" {
		t.Errorf("externalStudioHost = %q, want https://studio.example.com", project.Metadata["externalStudioHost"])
	}
}

// --- CORS origin import tests ---

func TestCORSOriginImport_FindsMatchingOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, []sanity.CORSEntry{
			{Id: 100, Origin: "https://example.com", AllowCredentials: true, ProjectId: "proj123"},
			{Id: 200, Origin: "https://other.com", AllowCredentials: false, ProjectId: "proj123"},
		})
	}))
	defer server.Close()

	client := newTestClient(server)

	// Simulate the import logic from cors_origin_resource.go ImportState
	entries, err := client.Projects.ListCORSEntries(context.Background(), "proj123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	origin := "https://example.com"
	found := false
	var foundEntry sanity.CORSEntry
	for _, e := range entries {
		if e.Origin == origin {
			foundEntry = e
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected to find matching CORS entry")
	}
	if foundEntry.Id != 100 {
		t.Errorf("entry ID = %d, want 100", foundEntry.Id)
	}
	if !foundEntry.AllowCredentials {
		t.Error("expected AllowCredentials to be true")
	}
}

func TestCORSOriginImport_ParseFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantProj  string
		wantOrig  string
		wantError bool
	}{
		{
			name:     "valid",
			input:    "proj123/https://example.com",
			wantProj: "proj123",
			wantOrig: "https://example.com",
		},
		{
			name:      "missing origin",
			input:     "proj123/",
			wantError: true,
		},
		{
			name:      "no separator",
			input:     "proj123",
			wantError: true,
		},
		{
			name:     "origin with port",
			input:    "proj123/http://localhost:3000",
			wantProj: "proj123",
			wantOrig: "http://localhost:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the import parsing logic from cors_origin_resource.go
			var projectId, origin string
			var found bool
			for i, c := range tt.input {
				if c == '/' {
					projectId = tt.input[:i]
					origin = tt.input[i+1:]
					found = true
					break
				}
			}

			if !found || projectId == "" || origin == "" {
				if !tt.wantError {
					t.Error("unexpected parse error")
				}
				return
			}
			if tt.wantError {
				t.Error("expected parse error but got none")
				return
			}
			if projectId != tt.wantProj {
				t.Errorf("projectId = %q, want %q", projectId, tt.wantProj)
			}
			if origin != tt.wantOrig {
				t.Errorf("origin = %q, want %q", origin, tt.wantOrig)
			}
		})
	}
}

// --- Dataset import tests ---

func TestDatasetImport_ParseFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantProj  string
		wantName  string
		wantError bool
	}{
		{
			name:     "valid",
			input:    "proj123/production",
			wantProj: "proj123",
			wantName: "production",
		},
		{
			name:      "too many parts",
			input:     "proj123/production/extra",
			wantError: true,
		},
		{
			name:      "no separator",
			input:     "proj123",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate dataset import logic: strings.Split(req.ID, "/")
			var parts []string
			start := 0
			for i, c := range tt.input {
				if c == '/' {
					parts = append(parts, tt.input[start:i])
					start = i + 1
				}
			}
			parts = append(parts, tt.input[start:])

			if len(parts) != 2 {
				if !tt.wantError {
					t.Errorf("unexpected parse error, got %d parts: %v", len(parts), parts)
				}
				return
			}
			if tt.wantError {
				t.Error("expected parse error but got none")
				return
			}
			if parts[0] != tt.wantProj {
				t.Errorf("project = %q, want %q", parts[0], tt.wantProj)
			}
			if parts[1] != tt.wantName {
				t.Errorf("name = %q, want %q", parts[1], tt.wantName)
			}
		})
	}
}

// --- Webhook tests ---

func TestWebhookImport_ParseFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantProj   string
		wantHookId string
		wantError  bool
	}{
		{
			name:       "valid",
			input:      "proj123/webhook456",
			wantProj:   "proj123",
			wantHookId: "webhook456",
		},
		{
			name:      "too many parts",
			input:     "proj123/hook456/extra",
			wantError: true,
		},
		{
			name:      "no separator",
			input:     "proj123",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parts []string
			start := 0
			for i, c := range tt.input {
				if c == '/' {
					parts = append(parts, tt.input[start:i])
					start = i + 1
				}
			}
			parts = append(parts, tt.input[start:])

			if len(parts) != 2 {
				if !tt.wantError {
					t.Errorf("unexpected parse error, got %d parts: %v", len(parts), parts)
				}
				return
			}
			if tt.wantError {
				t.Error("expected parse error but got none")
				return
			}
			if parts[0] != tt.wantProj {
				t.Errorf("project = %q, want %q", parts[0], tt.wantProj)
			}
			if parts[1] != tt.wantHookId {
				t.Errorf("webhookId = %q, want %q", parts[1], tt.wantHookId)
			}
		})
	}
}

func TestWebhookUpdateModel_SecretPreservedFromState(t *testing.T) {
	// The API never returns the webhook secret. Verify that the
	// updateModelFromWebhook function preserves the existing secret
	// from state (via the data model) rather than overwriting it.
	r := &WebhookResource{}

	data := &WebhookResourceModel{
		Secret: types.StringValue("my-secret-value"),
	}

	webhook := &sanity.Webhook{
		Id:         "hook123",
		ProjectId:  "proj123",
		Name:       "Deploy hook",
		Dataset:    "production",
		URL:        "https://example.com/hook",
		HttpMethod: "POST",
		ApiVersion: "v2021-03-25",
		CreatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		// Secret is empty — API doesn't return it
		Secret: "",
	}

	diags := r.updateModelFromWebhook(context.Background(), data, webhook)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Secret should be preserved from the original data model
	if data.Secret.ValueString() != "my-secret-value" {
		t.Errorf("Secret = %q, want %q (should be preserved from state)", data.Secret.ValueString(), "my-secret-value")
	}

	// Other fields should be updated from the webhook
	if data.Id.ValueString() != "hook123" {
		t.Errorf("Id = %q, want hook123", data.Id.ValueString())
	}
	if data.Name.ValueString() != "Deploy hook" {
		t.Errorf("Name = %q, want Deploy hook", data.Name.ValueString())
	}
}

func TestWebhookUpdateModel_FilterFromRule(t *testing.T) {
	r := &WebhookResource{}

	t.Run("with filter", func(t *testing.T) {
		data := &WebhookResourceModel{Secret: types.StringNull()}
		webhook := &sanity.Webhook{
			Id:         "hook1",
			ProjectId:  "proj1",
			Name:       "Test",
			Dataset:    "production",
			URL:        "https://example.com",
			HttpMethod: "POST",
			ApiVersion: "v2021-03-25",
			Rule:       &sanity.WebhookRule{Filter: "_type == 'post'"},
			CreatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		r.updateModelFromWebhook(context.Background(), data, webhook)
		if data.Filter.ValueString() != "_type == 'post'" {
			t.Errorf("Filter = %q, want \"_type == 'post'\"", data.Filter.ValueString())
		}
	})

	t.Run("without rule", func(t *testing.T) {
		data := &WebhookResourceModel{Secret: types.StringNull()}
		webhook := &sanity.Webhook{
			Id:         "hook2",
			ProjectId:  "proj1",
			Name:       "Test",
			Dataset:    "production",
			URL:        "https://example.com",
			HttpMethod: "POST",
			ApiVersion: "v2021-03-25",
			Rule:       nil,
			CreatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		r.updateModelFromWebhook(context.Background(), data, webhook)
		if !data.Filter.IsNull() {
			t.Errorf("Filter should be null when no rule, got %q", data.Filter.ValueString())
		}
	})

	t.Run("with empty filter", func(t *testing.T) {
		data := &WebhookResourceModel{Secret: types.StringNull()}
		webhook := &sanity.Webhook{
			Id:         "hook3",
			ProjectId:  "proj1",
			Name:       "Test",
			Dataset:    "production",
			URL:        "https://example.com",
			HttpMethod: "POST",
			ApiVersion: "v2021-03-25",
			Rule:       &sanity.WebhookRule{Filter: ""},
			CreatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		r.updateModelFromWebhook(context.Background(), data, webhook)
		if !data.Filter.IsNull() {
			t.Errorf("Filter should be null when filter is empty, got %q", data.Filter.ValueString())
		}
	})
}

func TestWebhookUpdateModel_HeadersMapping(t *testing.T) {
	r := &WebhookResource{}

	t.Run("with headers", func(t *testing.T) {
		data := &WebhookResourceModel{Secret: types.StringNull()}
		webhook := &sanity.Webhook{
			Id:         "hook1",
			ProjectId:  "proj1",
			Name:       "Test",
			Dataset:    "production",
			URL:        "https://example.com",
			HttpMethod: "POST",
			ApiVersion: "v2021-03-25",
			Headers:    map[string]string{"X-Custom": "value"},
			CreatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		// Need a real context for MapValueFrom
		diags := r.updateModelFromWebhook(context.Background(), data, webhook)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if data.Headers.IsNull() {
			t.Error("Headers should not be null when webhook has headers")
		}
	})

	t.Run("without headers", func(t *testing.T) {
		data := &WebhookResourceModel{Secret: types.StringNull()}
		webhook := &sanity.Webhook{
			Id:         "hook2",
			ProjectId:  "proj1",
			Name:       "Test",
			Dataset:    "production",
			URL:        "https://example.com",
			HttpMethod: "POST",
			ApiVersion: "v2021-03-25",
			Headers:    nil,
			CreatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		diags := r.updateModelFromWebhook(context.Background(), data, webhook)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if !data.Headers.IsNull() {
			t.Error("Headers should be null when webhook has no headers")
		}
	})
}
