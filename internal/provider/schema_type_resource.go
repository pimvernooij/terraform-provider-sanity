package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tessellator/terraform-provider-sanity/internal/schemaclient"
)

// schemaMutexes prevents concurrent read-modify-write on the same schema document.
var (
	schemaMutexes   = map[string]*sync.Mutex{}
	schemaMutexesMu sync.Mutex
)

func getSchemaMutex(key string) *sync.Mutex {
	schemaMutexesMu.Lock()
	defer schemaMutexesMu.Unlock()
	if schemaMutexes[key] == nil {
		schemaMutexes[key] = &sync.Mutex{}
	}
	return schemaMutexes[key]
}

var _ resource.Resource = &SchemaTypeResource{}
var _ resource.ResourceWithImportState = &SchemaTypeResource{}

func NewSchemaTypeResource() resource.Resource {
	return &SchemaTypeResource{}
}

type SchemaTypeResource struct {
	client *schemaclient.Client
}

type SchemaTypeResourceModel struct {
	ID             types.String           `tfsdk:"id"`
	ProjectID      types.String           `tfsdk:"project_id"`
	Dataset        types.String           `tfsdk:"dataset"`
	WorkspaceName  types.String           `tfsdk:"workspace_name"`
	Version        types.String           `tfsdk:"version"`
	Tag            types.String           `tfsdk:"tag"`
	Name           types.String           `tfsdk:"name"`
	Type           types.String           `tfsdk:"type"`
	Title          types.String           `tfsdk:"title"`
	Description    types.String           `tfsdk:"description"`
	Fields         []SchemaTypeFieldModel `tfsdk:"field"`
}

type SchemaTypeFieldModel struct {
	Name        types.String              `tfsdk:"name"`
	Type        types.String              `tfsdk:"type"`
	Title       types.String              `tfsdk:"title"`
	Description types.String              `tfsdk:"description"`
	Hidden      types.Bool                `tfsdk:"hidden"`
	ReadOnly    types.Bool                `tfsdk:"readonly"`
	Options     jsontypes.Normalized      `tfsdk:"options"`
	Of          jsontypes.Normalized      `tfsdk:"of"`
	To          jsontypes.Normalized      `tfsdk:"to"`
	Fields      []SchemaTypeNestedFieldModel `tfsdk:"field"`
	FieldsJSON  jsontypes.Normalized      `tfsdk:"fields_json"`
}

type SchemaTypeNestedFieldModel struct {
	Name        types.String         `tfsdk:"name"`
	Type        types.String         `tfsdk:"type"`
	Title       types.String         `tfsdk:"title"`
	Description types.String         `tfsdk:"description"`
	Hidden      types.Bool           `tfsdk:"hidden"`
	ReadOnly    types.Bool           `tfsdk:"readonly"`
	Options     jsontypes.Normalized `tfsdk:"options"`
	Of          jsontypes.Normalized `tfsdk:"of"`
	To          jsontypes.Normalized `tfsdk:"to"`
	FieldsJSON  jsontypes.Normalized `tfsdk:"fields_json"`
}

// nestedFieldAttributes returns the schema attributes shared by both field levels.
func nestedFieldAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The field name.",
		},
		"type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The field type (e.g. `string`, `text`, `image`, `reference`, `array`, `object`, `block`).",
		},
		"title": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "A human-readable title for the field.",
		},
		"description": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "A description shown below the field in the studio.",
		},
		"hidden": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether the field is hidden in the studio.",
		},
		"readonly": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether the field is read-only in the studio.",
		},
		"options": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Type-specific options as JSON (e.g. `jsonencode({ source = \"title\" })` for a slug field).",
			CustomType:          jsontypes.NormalizedType{},
		},
		"of": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Array item types as JSON (e.g. `jsonencode([{ type = \"block\" }])`).",
			CustomType:          jsontypes.NormalizedType{},
		},
		"to": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Reference target types as JSON (e.g. `jsonencode([{ type = \"author\" }])`).",
			CustomType:          jsontypes.NormalizedType{},
		},
		"fields_json": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Escape hatch: nested fields as a JSON array for deeply nested structures that exceed the block nesting depth.",
			CustomType:          jsontypes.NormalizedType{},
		},
	}
}

func (r *SchemaTypeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema_type"
}

func (r *SchemaTypeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single content type within a Sanity workspace schema. Each resource represents one document or object type. The provider handles read-modify-write merging with the full schema automatically.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite ID in the form `project_id/dataset/workspace_name/type_name` (or with `/tag` suffix).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dataset": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The dataset name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The workspace name, e.g. `default`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The schema version, e.g. `2025-05-01`.",
			},
			"tag": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "An optional tag for the schema.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type name (e.g. `post`, `author`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The schema type kind (e.g. `document`, `object`).",
			},
			"title": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A human-readable title for the type.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A description for the type.",
			},
		},
		Blocks: map[string]schema.Block{
			"field": schema.ListNestedBlock{
				MarkdownDescription: "A field definition for this type.",
				NestedObject: schema.NestedBlockObject{
					Attributes: nestedFieldAttributes(),
					Blocks: map[string]schema.Block{
						"field": schema.ListNestedBlock{
							MarkdownDescription: "A nested field (for image, object, or other compound types).",
							NestedObject: schema.NestedBlockObject{
								Attributes: nestedFieldAttributes(),
							},
						},
					},
				},
			},
		},
	}
}

func (r *SchemaTypeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = clients.SchemaClient
}

func (r *SchemaTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SchemaTypeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mu := getSchemaMutex(data.schemaKey())
	mu.Lock()
	defer mu.Unlock()

	typeDef := data.toTypeDefinition()

	err := r.mergeType(ctx, &data, typeDef)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create schema type: %s", err))
		return
	}

	data.ID = types.StringValue(data.compositeID())
	tflog.Trace(ctx, "created a sanity schema type", map[string]interface{}{"name": data.Name.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SchemaTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SchemaTypeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	schemaID := schemaclient.SchemaID(data.WorkspaceName.ValueString(), data.Tag.ValueString())
	doc, err := r.client.GetSchema(ctx, data.ProjectID.ValueString(), data.Dataset.ValueString(), schemaID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read schema: %s", err))
		return
	}

	typeDef, err := findTypeInSchema(doc.Schema, data.Name.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.updateFromTypeDef(typeDef)
	data.Version = types.StringValue(doc.Version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SchemaTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SchemaTypeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mu := getSchemaMutex(data.schemaKey())
	mu.Lock()
	defer mu.Unlock()

	typeDef := data.toTypeDefinition()

	err := r.mergeType(ctx, &data, typeDef)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update schema type: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SchemaTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SchemaTypeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mu := getSchemaMutex(data.schemaKey())
	mu.Lock()
	defer mu.Unlock()

	projectID := data.ProjectID.ValueString()
	dataset := data.Dataset.ValueString()
	schemaID := schemaclient.SchemaID(data.WorkspaceName.ValueString(), data.Tag.ValueString())

	doc, err := r.client.GetSchema(ctx, projectID, dataset, schemaID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read schema for deletion: %s", err))
		return
	}

	var schemaTypes []map[string]interface{}
	if err := json.Unmarshal(doc.Schema, &schemaTypes); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse schema: %s", err))
		return
	}

	// Remove this type
	typeName := data.Name.ValueString()
	filtered := make([]map[string]interface{}, 0, len(schemaTypes))
	for _, t := range schemaTypes {
		if name, _ := t["name"].(string); name != typeName {
			filtered = append(filtered, t)
		}
	}

	newSchema, _ := json.Marshal(filtered)
	putReq := &schemaclient.PutSchemasRequest{
		Workspace: schemaclient.SchemaWorkspace{Name: data.WorkspaceName.ValueString()},
		Schema:    json.RawMessage(newSchema),
		Version:   data.Version.ValueString(),
		Tag:       data.Tag.ValueString(),
	}

	_, err = r.client.PutSchemas(ctx, projectID, dataset, putReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove type from schema: %s", err))
		return
	}
}

func (r *SchemaTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: project_id/dataset/workspace_name/type_name or project_id/dataset/workspace_name/type_name/tag
	parts := strings.Split(req.ID, "/")
	if len(parts) < 4 || len(parts) > 5 {
		resp.Diagnostics.AddError(
			"Import Error",
			"The import identifier should be project_id/dataset/workspace_name/type_name or project_id/dataset/workspace_name/type_name/tag",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dataset"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_name"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[3])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	if len(parts) == 5 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag"), parts[4])...)
	}
}

// mergeType reads the current schema, adds/replaces this type, and PUTs back.
func (r *SchemaTypeResource) mergeType(ctx context.Context, data *SchemaTypeResourceModel, typeDef map[string]interface{}) error {
	projectID := data.ProjectID.ValueString()
	dataset := data.Dataset.ValueString()
	schemaID := schemaclient.SchemaID(data.WorkspaceName.ValueString(), data.Tag.ValueString())

	var schemaTypes []map[string]interface{}

	// Try to read existing schema; if it doesn't exist, start empty
	doc, err := r.client.GetSchema(ctx, projectID, dataset, schemaID)
	if err == nil {
		if err := json.Unmarshal(doc.Schema, &schemaTypes); err != nil {
			return fmt.Errorf("parsing existing schema: %w", err)
		}
	}

	// Replace or append
	typeName := data.Name.ValueString()
	found := false
	for i, t := range schemaTypes {
		if name, _ := t["name"].(string); name == typeName {
			schemaTypes[i] = typeDef
			found = true
			break
		}
	}
	if !found {
		schemaTypes = append(schemaTypes, typeDef)
	}

	newSchema, _ := json.Marshal(schemaTypes)
	putReq := &schemaclient.PutSchemasRequest{
		Workspace: schemaclient.SchemaWorkspace{Name: data.WorkspaceName.ValueString()},
		Schema:    json.RawMessage(newSchema),
		Version:   data.Version.ValueString(),
		Tag:       data.Tag.ValueString(),
	}

	_, err = r.client.PutSchemas(ctx, projectID, dataset, putReq)
	return err
}

// schemaKey returns a unique key for the mutex map.
func (data *SchemaTypeResourceModel) schemaKey() string {
	return fmt.Sprintf("%s/%s/%s/%s",
		data.ProjectID.ValueString(),
		data.Dataset.ValueString(),
		data.WorkspaceName.ValueString(),
		data.Tag.ValueString(),
	)
}

// compositeID returns the Terraform resource ID.
func (data *SchemaTypeResourceModel) compositeID() string {
	base := fmt.Sprintf("%s/%s/%s/%s",
		data.ProjectID.ValueString(),
		data.Dataset.ValueString(),
		data.WorkspaceName.ValueString(),
		data.Name.ValueString(),
	)
	if !data.Tag.IsNull() && data.Tag.ValueString() != "" {
		return base + "/" + data.Tag.ValueString()
	}
	return base
}

// toTypeDefinition converts the HCL model to a JSON-compatible map.
func (data *SchemaTypeResourceModel) toTypeDefinition() map[string]interface{} {
	typeDef := map[string]interface{}{
		"name": data.Name.ValueString(),
		"type": data.Type.ValueString(),
	}
	if !data.Title.IsNull() && data.Title.ValueString() != "" {
		typeDef["title"] = data.Title.ValueString()
	}
	if !data.Description.IsNull() && data.Description.ValueString() != "" {
		typeDef["description"] = data.Description.ValueString()
	}
	if len(data.Fields) > 0 {
		typeDef["fields"] = fieldsToJSON(data.Fields)
	}
	return typeDef
}

func fieldsToJSON(fields []SchemaTypeFieldModel) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(fields))
	for _, f := range fields {
		fd := fieldBaseToJSON(f.Name, f.Type, f.Title, f.Description, f.Hidden, f.ReadOnly, f.Options, f.Of, f.To)

		// Nested field blocks (1 level)
		if len(f.Fields) > 0 {
			fd["fields"] = nestedFieldsToJSON(f.Fields)
		}

		// Escape hatch
		if !f.FieldsJSON.IsNull() && !f.FieldsJSON.IsUnknown() {
			var nested []interface{}
			if err := json.Unmarshal([]byte(f.FieldsJSON.ValueString()), &nested); err == nil {
				fd["fields"] = nested
			}
		}

		result = append(result, fd)
	}
	return result
}

func nestedFieldsToJSON(fields []SchemaTypeNestedFieldModel) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(fields))
	for _, f := range fields {
		fd := fieldBaseToJSON(f.Name, f.Type, f.Title, f.Description, f.Hidden, f.ReadOnly, f.Options, f.Of, f.To)

		if !f.FieldsJSON.IsNull() && !f.FieldsJSON.IsUnknown() {
			var nested []interface{}
			if err := json.Unmarshal([]byte(f.FieldsJSON.ValueString()), &nested); err == nil {
				fd["fields"] = nested
			}
		}

		result = append(result, fd)
	}
	return result
}

func fieldBaseToJSON(name, typ, title, description types.String, hidden, readonly types.Bool, options, of, to jsontypes.Normalized) map[string]interface{} {
	fd := map[string]interface{}{
		"name": name.ValueString(),
		"type": typ.ValueString(),
	}
	if !title.IsNull() && title.ValueString() != "" {
		fd["title"] = title.ValueString()
	}
	if !description.IsNull() && description.ValueString() != "" {
		fd["description"] = description.ValueString()
	}
	if !hidden.IsNull() && hidden.ValueBool() {
		fd["hidden"] = true
	}
	if !readonly.IsNull() && readonly.ValueBool() {
		fd["readOnly"] = true
	}
	if !options.IsNull() && !options.IsUnknown() {
		var v interface{}
		if err := json.Unmarshal([]byte(options.ValueString()), &v); err == nil {
			fd["options"] = v
		}
	}
	if !of.IsNull() && !of.IsUnknown() {
		var v interface{}
		if err := json.Unmarshal([]byte(of.ValueString()), &v); err == nil {
			fd["of"] = v
		}
	}
	if !to.IsNull() && !to.IsUnknown() {
		var v interface{}
		if err := json.Unmarshal([]byte(to.ValueString()), &v); err == nil {
			fd["to"] = v
		}
	}
	return fd
}

// findTypeInSchema finds a type by name in the schema JSON array.
func findTypeInSchema(schemaJSON json.RawMessage, typeName string) (map[string]interface{}, error) {
	var types []map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &types); err != nil {
		return nil, err
	}
	for _, t := range types {
		if name, _ := t["name"].(string); name == typeName {
			return t, nil
		}
	}
	return nil, fmt.Errorf("type %q not found in schema", typeName)
}

// updateFromTypeDef updates the model from a JSON type definition read from the API.
func (data *SchemaTypeResourceModel) updateFromTypeDef(typeDef map[string]interface{}) {
	if v, ok := typeDef["name"].(string); ok {
		data.Name = types.StringValue(v)
	}
	if v, ok := typeDef["type"].(string); ok {
		data.Type = types.StringValue(v)
	}
	if v, ok := typeDef["title"].(string); ok {
		data.Title = types.StringValue(v)
	} else {
		data.Title = types.StringNull()
	}
	if v, ok := typeDef["description"].(string); ok {
		data.Description = types.StringValue(v)
	} else {
		data.Description = types.StringNull()
	}

	if fields, ok := typeDef["fields"].([]interface{}); ok {
		data.Fields = jsonFieldsToModel(fields)
	}

	data.ID = types.StringValue(data.compositeID())
}

func jsonFieldsToModel(fields []interface{}) []SchemaTypeFieldModel {
	result := make([]SchemaTypeFieldModel, 0, len(fields))
	for _, raw := range fields {
		f, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		model := SchemaTypeFieldModel{}
		model.Name = stringFromMap(f, "name")
		model.Type = stringFromMap(f, "type")
		model.Title = optionalStringFromMap(f, "title")
		model.Description = optionalStringFromMap(f, "description")
		model.Hidden = optionalBoolFromMap(f, "hidden")
		model.ReadOnly = optionalBoolFromMap(f, "readOnly")
		model.Options = jsonAttrFromMap(f, "options")
		model.Of = jsonAttrFromMap(f, "of")
		model.To = jsonAttrFromMap(f, "to")

		// Nested fields become nested field blocks
		if nested, ok := f["fields"].([]interface{}); ok {
			model.Fields = jsonNestedFieldsToModel(nested)
		}

		result = append(result, model)
	}
	return result
}

func jsonNestedFieldsToModel(fields []interface{}) []SchemaTypeNestedFieldModel {
	result := make([]SchemaTypeNestedFieldModel, 0, len(fields))
	for _, raw := range fields {
		f, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		model := SchemaTypeNestedFieldModel{}
		model.Name = stringFromMap(f, "name")
		model.Type = stringFromMap(f, "type")
		model.Title = optionalStringFromMap(f, "title")
		model.Description = optionalStringFromMap(f, "description")
		model.Hidden = optionalBoolFromMap(f, "hidden")
		model.ReadOnly = optionalBoolFromMap(f, "readOnly")
		model.Options = jsonAttrFromMap(f, "options")
		model.Of = jsonAttrFromMap(f, "of")
		model.To = jsonAttrFromMap(f, "to")

		// Deeper nesting goes to fields_json
		if nested, ok := f["fields"].([]interface{}); ok && len(nested) > 0 {
			b, _ := json.Marshal(nested)
			model.FieldsJSON = jsontypes.NewNormalizedValue(string(b))
		} else {
			model.FieldsJSON = jsontypes.NewNormalizedNull()
		}

		result = append(result, model)
	}
	return result
}

func stringFromMap(m map[string]interface{}, key string) types.String {
	if v, ok := m[key].(string); ok {
		return types.StringValue(v)
	}
	return types.StringValue("")
}

func optionalStringFromMap(m map[string]interface{}, key string) types.String {
	if v, ok := m[key].(string); ok && v != "" {
		return types.StringValue(v)
	}
	return types.StringNull()
}

func optionalBoolFromMap(m map[string]interface{}, key string) types.Bool {
	if v, ok := m[key].(bool); ok && v {
		return types.BoolValue(true)
	}
	return types.BoolNull()
}

func jsonAttrFromMap(m map[string]interface{}, key string) jsontypes.Normalized {
	v, ok := m[key]
	if !ok || v == nil {
		return jsontypes.NewNormalizedNull()
	}
	b, err := json.Marshal(v)
	if err != nil {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(b))
}
