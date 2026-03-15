package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSchemaTypeCompositeID(t *testing.T) {
	tests := []struct {
		name     string
		model    SchemaTypeResourceModel
		expected string
	}{
		{
			name: "without tag",
			model: SchemaTypeResourceModel{
				ProjectID:     types.StringValue("proj123"),
				Dataset:       types.StringValue("production"),
				WorkspaceName: types.StringValue("default"),
				Name:          types.StringValue("article"),
				Tag:           types.StringNull(),
			},
			expected: "proj123/production/default/article",
		},
		{
			name: "with tag",
			model: SchemaTypeResourceModel{
				ProjectID:     types.StringValue("proj123"),
				Dataset:       types.StringValue("production"),
				WorkspaceName: types.StringValue("default"),
				Name:          types.StringValue("article"),
				Tag:           types.StringValue("v1"),
			},
			expected: "proj123/production/default/article/v1",
		},
		{
			name: "empty tag treated as no tag",
			model: SchemaTypeResourceModel{
				ProjectID:     types.StringValue("proj123"),
				Dataset:       types.StringValue("production"),
				WorkspaceName: types.StringValue("default"),
				Name:          types.StringValue("article"),
				Tag:           types.StringValue(""),
			},
			expected: "proj123/production/default/article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.compositeID()
			if got != tt.expected {
				t.Errorf("compositeID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSchemaTypeSchemaKey(t *testing.T) {
	model := SchemaTypeResourceModel{
		ProjectID:     types.StringValue("proj123"),
		Dataset:       types.StringValue("production"),
		WorkspaceName: types.StringValue("default"),
		Tag:           types.StringValue("v1"),
	}

	expected := "proj123/production/default/v1"
	got := model.schemaKey()
	if got != expected {
		t.Errorf("schemaKey() = %q, want %q", got, expected)
	}
}

func TestSchemaTypeToTypeDefinition(t *testing.T) {
	model := SchemaTypeResourceModel{
		Name:        types.StringValue("article"),
		Type:        types.StringValue("document"),
		Title:       types.StringValue("Article"),
		Description: types.StringValue("A blog article"),
		Fields: []SchemaTypeFieldModel{
			{
				Name:        types.StringValue("title"),
				Type:        types.StringValue("string"),
				Title:       types.StringValue("Title"),
				Description: types.StringNull(),
				Hidden:      types.BoolNull(),
				ReadOnly:    types.BoolNull(),
				Options:     jsontypes.NewNormalizedNull(),
				Of:          jsontypes.NewNormalizedNull(),
				To:          jsontypes.NewNormalizedNull(),
				FieldsJSON:  jsontypes.NewNormalizedNull(),
			},
		},
	}

	typeDef := model.toTypeDefinition()

	if typeDef["name"] != "article" {
		t.Errorf("name = %v, want article", typeDef["name"])
	}
	if typeDef["type"] != "document" {
		t.Errorf("type = %v, want document", typeDef["type"])
	}
	if typeDef["title"] != "Article" {
		t.Errorf("title = %v, want Article", typeDef["title"])
	}
	if typeDef["description"] != "A blog article" {
		t.Errorf("description = %v, want A blog article", typeDef["description"])
	}

	fields, ok := typeDef["fields"].([]map[string]interface{})
	if !ok {
		t.Fatalf("fields is not []map[string]interface{}, got %T", typeDef["fields"])
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0]["name"] != "title" {
		t.Errorf("field name = %v, want title", fields[0]["name"])
	}
}

func TestSchemaTypeToTypeDefinitionOmitsEmptyOptionals(t *testing.T) {
	model := SchemaTypeResourceModel{
		Name:        types.StringValue("tag"),
		Type:        types.StringValue("document"),
		Title:       types.StringNull(),
		Description: types.StringNull(),
	}

	typeDef := model.toTypeDefinition()

	if _, ok := typeDef["title"]; ok {
		t.Error("title should be omitted when null")
	}
	if _, ok := typeDef["description"]; ok {
		t.Error("description should be omitted when null")
	}
	if _, ok := typeDef["fields"]; ok {
		t.Error("fields should be omitted when empty")
	}
}

func TestFieldBaseToJSON(t *testing.T) {
	t.Run("minimal field", func(t *testing.T) {
		fd := fieldBaseToJSON(
			types.StringValue("title"),
			types.StringValue("string"),
			types.StringNull(),
			types.StringNull(),
			types.BoolNull(),
			types.BoolNull(),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
		)

		if fd["name"] != "title" {
			t.Errorf("name = %v, want title", fd["name"])
		}
		if fd["type"] != "string" {
			t.Errorf("type = %v, want string", fd["type"])
		}
		if _, ok := fd["title"]; ok {
			t.Error("title should be omitted")
		}
		if _, ok := fd["hidden"]; ok {
			t.Error("hidden should be omitted")
		}
	})

	t.Run("field with options", func(t *testing.T) {
		fd := fieldBaseToJSON(
			types.StringValue("slug"),
			types.StringValue("slug"),
			types.StringValue("Slug"),
			types.StringNull(),
			types.BoolNull(),
			types.BoolNull(),
			jsontypes.NewNormalizedValue(`{"source":"title"}`),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
		)

		opts, ok := fd["options"].(map[string]interface{})
		if !ok {
			t.Fatalf("options is not map[string]interface{}, got %T", fd["options"])
		}
		if opts["source"] != "title" {
			t.Errorf("options.source = %v, want title", opts["source"])
		}
	})

	t.Run("field with hidden and readonly", func(t *testing.T) {
		fd := fieldBaseToJSON(
			types.StringValue("internal"),
			types.StringValue("string"),
			types.StringNull(),
			types.StringNull(),
			types.BoolValue(true),
			types.BoolValue(true),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
		)

		if fd["hidden"] != true {
			t.Error("hidden should be true")
		}
		if fd["readOnly"] != true {
			t.Error("readOnly should be true")
		}
	})

	t.Run("hidden false is omitted", func(t *testing.T) {
		fd := fieldBaseToJSON(
			types.StringValue("visible"),
			types.StringValue("string"),
			types.StringNull(),
			types.StringNull(),
			types.BoolValue(false),
			types.BoolValue(false),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedNull(),
		)

		if _, ok := fd["hidden"]; ok {
			t.Error("hidden=false should be omitted")
		}
		if _, ok := fd["readOnly"]; ok {
			t.Error("readOnly=false should be omitted")
		}
	})

	t.Run("field with of and to", func(t *testing.T) {
		fd := fieldBaseToJSON(
			types.StringValue("refs"),
			types.StringValue("array"),
			types.StringNull(),
			types.StringNull(),
			types.BoolNull(),
			types.BoolNull(),
			jsontypes.NewNormalizedNull(),
			jsontypes.NewNormalizedValue(`[{"type":"reference"}]`),
			jsontypes.NewNormalizedValue(`[{"type":"author"}]`),
		)

		if fd["of"] == nil {
			t.Error("of should be set")
		}
		if fd["to"] == nil {
			t.Error("to should be set")
		}
	})
}

func TestFieldsToJSON(t *testing.T) {
	fields := []SchemaTypeFieldModel{
		{
			Name:       types.StringValue("title"),
			Type:       types.StringValue("string"),
			Title:      types.StringValue("Title"),
			Description: types.StringNull(),
			Hidden:     types.BoolNull(),
			ReadOnly:   types.BoolNull(),
			Options:    jsontypes.NewNormalizedNull(),
			Of:         jsontypes.NewNormalizedNull(),
			To:         jsontypes.NewNormalizedNull(),
			FieldsJSON: jsontypes.NewNormalizedNull(),
			Fields: []SchemaTypeNestedFieldModel{
				{
					Name:       types.StringValue("subtitle"),
					Type:       types.StringValue("string"),
					Title:      types.StringNull(),
					Description: types.StringNull(),
					Hidden:     types.BoolNull(),
					ReadOnly:   types.BoolNull(),
					Options:    jsontypes.NewNormalizedNull(),
					Of:         jsontypes.NewNormalizedNull(),
					To:         jsontypes.NewNormalizedNull(),
					FieldsJSON: jsontypes.NewNormalizedNull(),
				},
			},
		},
	}

	result := fieldsToJSON(fields)
	if len(result) != 1 {
		t.Fatalf("expected 1 field, got %d", len(result))
	}

	nested, ok := result[0]["fields"].([]map[string]interface{})
	if !ok {
		t.Fatalf("nested fields is not []map[string]interface{}, got %T", result[0]["fields"])
	}
	if len(nested) != 1 {
		t.Fatalf("expected 1 nested field, got %d", len(nested))
	}
	if nested[0]["name"] != "subtitle" {
		t.Errorf("nested field name = %v, want subtitle", nested[0]["name"])
	}
}

func TestFieldsJSONEscapeHatch(t *testing.T) {
	fields := []SchemaTypeFieldModel{
		{
			Name:       types.StringValue("complex"),
			Type:       types.StringValue("object"),
			Title:      types.StringNull(),
			Description: types.StringNull(),
			Hidden:     types.BoolNull(),
			ReadOnly:   types.BoolNull(),
			Options:    jsontypes.NewNormalizedNull(),
			Of:         jsontypes.NewNormalizedNull(),
			To:         jsontypes.NewNormalizedNull(),
			FieldsJSON: jsontypes.NewNormalizedValue(`[{"name":"deep","type":"string"}]`),
		},
	}

	result := fieldsToJSON(fields)
	nested, ok := result[0]["fields"].([]interface{})
	if !ok {
		t.Fatalf("fields_json should produce []interface{}, got %T", result[0]["fields"])
	}
	if len(nested) != 1 {
		t.Fatalf("expected 1 nested field, got %d", len(nested))
	}
}

func TestUpdateFromTypeDef(t *testing.T) {
	typeDef := map[string]interface{}{
		"name":  "article",
		"type":  "document",
		"title": "Article",
		"fields": []interface{}{
			map[string]interface{}{
				"name":  "title",
				"type":  "string",
				"title": "Title",
			},
			map[string]interface{}{
				"name":   "slug",
				"type":   "slug",
				"hidden": true,
				"options": map[string]interface{}{
					"source": "title",
				},
			},
		},
	}

	data := &SchemaTypeResourceModel{
		ProjectID:     types.StringValue("proj123"),
		Dataset:       types.StringValue("production"),
		WorkspaceName: types.StringValue("default"),
		Tag:           types.StringNull(),
	}

	data.updateFromTypeDef(typeDef)

	if data.Name.ValueString() != "article" {
		t.Errorf("Name = %q, want article", data.Name.ValueString())
	}
	if data.Type.ValueString() != "document" {
		t.Errorf("Type = %q, want document", data.Type.ValueString())
	}
	if data.Title.ValueString() != "Article" {
		t.Errorf("Title = %q, want Article", data.Title.ValueString())
	}
	if !data.Description.IsNull() {
		t.Error("Description should be null when not in typedef")
	}
	if len(data.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(data.Fields))
	}
	if data.Fields[0].Name.ValueString() != "title" {
		t.Errorf("field 0 name = %q, want title", data.Fields[0].Name.ValueString())
	}
	if data.Fields[1].Name.ValueString() != "slug" {
		t.Errorf("field 1 name = %q, want slug", data.Fields[1].Name.ValueString())
	}
	if data.Fields[1].Hidden.IsNull() || !data.Fields[1].Hidden.ValueBool() {
		t.Error("field 1 hidden should be true")
	}
	if data.Fields[1].Options.IsNull() {
		t.Error("field 1 options should be set")
	}
}

func TestUpdateFromTypeDefWithNestedFields(t *testing.T) {
	typeDef := map[string]interface{}{
		"name": "article",
		"type": "document",
		"fields": []interface{}{
			map[string]interface{}{
				"name": "mainImage",
				"type": "image",
				"fields": []interface{}{
					map[string]interface{}{
						"name": "alt",
						"type": "string",
					},
				},
			},
		},
	}

	data := &SchemaTypeResourceModel{
		ProjectID:     types.StringValue("proj123"),
		Dataset:       types.StringValue("production"),
		WorkspaceName: types.StringValue("default"),
		Tag:           types.StringNull(),
	}

	data.updateFromTypeDef(typeDef)

	if len(data.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(data.Fields))
	}
	if len(data.Fields[0].Fields) != 1 {
		t.Fatalf("expected 1 nested field, got %d", len(data.Fields[0].Fields))
	}
	if data.Fields[0].Fields[0].Name.ValueString() != "alt" {
		t.Errorf("nested field name = %q, want alt", data.Fields[0].Fields[0].Name.ValueString())
	}
}

func TestFindTypeInSchema(t *testing.T) {
	schema := json.RawMessage(`[
		{"name": "article", "type": "document"},
		{"name": "author", "type": "document"}
	]`)

	t.Run("found", func(t *testing.T) {
		typeDef, err := findTypeInSchema(schema, "author")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if typeDef["name"] != "author" {
			t.Errorf("name = %v, want author", typeDef["name"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := findTypeInSchema(schema, "missing")
		if err == nil {
			t.Error("expected error for missing type")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := findTypeInSchema(json.RawMessage(`invalid`), "article")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	// Build a model, convert to JSON, parse back, verify equality
	original := &SchemaTypeResourceModel{
		ProjectID:     types.StringValue("proj123"),
		Dataset:       types.StringValue("production"),
		WorkspaceName: types.StringValue("default"),
		Tag:           types.StringNull(),
		Name:          types.StringValue("article"),
		Type:          types.StringValue("document"),
		Title:         types.StringValue("Article"),
		Description:   types.StringNull(),
		Fields: []SchemaTypeFieldModel{
			{
				Name:       types.StringValue("title"),
				Type:       types.StringValue("string"),
				Title:      types.StringValue("Title"),
				Description: types.StringNull(),
				Hidden:     types.BoolNull(),
				ReadOnly:   types.BoolNull(),
				Options:    jsontypes.NewNormalizedNull(),
				Of:         jsontypes.NewNormalizedNull(),
				To:         jsontypes.NewNormalizedNull(),
				FieldsJSON: jsontypes.NewNormalizedNull(),
				Fields: []SchemaTypeNestedFieldModel{},
			},
			{
				Name:       types.StringValue("slug"),
				Type:       types.StringValue("slug"),
				Title:      types.StringValue("Slug"),
				Description: types.StringNull(),
				Hidden:     types.BoolNull(),
				ReadOnly:   types.BoolNull(),
				Options:    jsontypes.NewNormalizedValue(`{"source":"title"}`),
				Of:         jsontypes.NewNormalizedNull(),
				To:         jsontypes.NewNormalizedNull(),
				FieldsJSON: jsontypes.NewNormalizedNull(),
				Fields: []SchemaTypeNestedFieldModel{},
			},
		},
	}

	// Convert to type definition (as would be sent to API)
	typeDef := original.toTypeDefinition()

	// Simulate what the API would return: marshal then unmarshal
	b, err := json.Marshal(typeDef)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Update a new model from the parsed response
	result := &SchemaTypeResourceModel{
		ProjectID:     types.StringValue("proj123"),
		Dataset:       types.StringValue("production"),
		WorkspaceName: types.StringValue("default"),
		Tag:           types.StringNull(),
	}
	result.updateFromTypeDef(parsed)

	// Verify key fields match
	if result.Name.ValueString() != original.Name.ValueString() {
		t.Errorf("Name mismatch: got %q, want %q", result.Name.ValueString(), original.Name.ValueString())
	}
	if result.Type.ValueString() != original.Type.ValueString() {
		t.Errorf("Type mismatch: got %q, want %q", result.Type.ValueString(), original.Type.ValueString())
	}
	if result.Title.ValueString() != original.Title.ValueString() {
		t.Errorf("Title mismatch: got %q, want %q", result.Title.ValueString(), original.Title.ValueString())
	}
	if len(result.Fields) != len(original.Fields) {
		t.Fatalf("field count mismatch: got %d, want %d", len(result.Fields), len(original.Fields))
	}
	if result.Fields[1].Options.IsNull() {
		t.Error("slug options should not be null after round-trip")
	}
}
