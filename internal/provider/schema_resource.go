package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

var _ resource.Resource = &SchemaResource{}
var _ resource.ResourceWithImportState = &SchemaResource{}

func NewSchemaResource() resource.Resource {
	return &SchemaResource{}
}

type SchemaResource struct {
	client *schemaclient.Client
}

type SchemaResourceModel struct {
	ID             types.String         `tfsdk:"id"`
	ProjectID      types.String         `tfsdk:"project_id"`
	Dataset        types.String         `tfsdk:"dataset"`
	WorkspaceName  types.String         `tfsdk:"workspace_name"`
	WorkspaceTitle types.String         `tfsdk:"workspace_title"`
	Version        types.String         `tfsdk:"version"`
	Tag            types.String         `tfsdk:"tag"`
	Schema         jsontypes.Normalized `tfsdk:"schema"`
}

func (r *SchemaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (r *SchemaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Sanity content schema for a workspace. The schema defines the document types and their fields available in Sanity Studio. Uses the Sanity Schema API to deploy full workspace schemas.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The schema document ID, e.g. `_.schemas.default`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The project ID that this schema belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dataset": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The dataset this schema is deployed to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The workspace name for the schema, e.g. `default`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_title": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable title for the workspace.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"schema": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The schema definition as a JSON array of type objects. Use `jsonencode()` or `file()` to provide the value.",
				CustomType:          jsontypes.NormalizedType{},
			},
		},
	}
}

func (r *SchemaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	putReq := &schemaclient.SchemaEntry{
		Workspace: schemaclient.SchemaWorkspace{
			Name:  data.WorkspaceName.ValueString(),
			Title: data.WorkspaceTitle.ValueString(),
		},
		Schema:  json.RawMessage(data.Schema.ValueString()),
		Version: data.Version.ValueString(),
		Tag:     data.Tag.ValueString(),
	}

	doc, err := r.client.PutSchemas(ctx, data.ProjectID.ValueString(), data.Dataset.ValueString(), putReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create schema: %s", err))
		return
	}

	r.updateModelFromDoc(&data, doc)

	tflog.Trace(ctx, "created a sanity schema", map[string]interface{}{"id": doc.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	doc, err := r.client.GetSchema(ctx, data.ProjectID.ValueString(), data.Dataset.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read schema: %s", err))
		return
	}

	r.updateModelFromDoc(&data, doc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	putReq := &schemaclient.SchemaEntry{
		Workspace: schemaclient.SchemaWorkspace{
			Name:  data.WorkspaceName.ValueString(),
			Title: data.WorkspaceTitle.ValueString(),
		},
		Schema:  json.RawMessage(data.Schema.ValueString()),
		Version: data.Version.ValueString(),
		Tag:     data.Tag.ValueString(),
	}

	doc, err := r.client.PutSchemas(ctx, data.ProjectID.ValueString(), data.Dataset.ValueString(), putReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update schema: %s", err))
		return
	}

	r.updateModelFromDoc(&data, doc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteSchema(ctx, data.ProjectID.ValueString(), data.Dataset.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete schema: %s", err))
		return
	}
}

func (r *SchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: project_id/dataset/workspace_name or project_id/dataset/workspace_name/tag
	parts := strings.Split(req.ID, "/")
	if len(parts) < 3 || len(parts) > 4 {
		resp.Diagnostics.AddError(
			"Import Error",
			"The import identifier for a schema should be in the form project_id/dataset/workspace_name or project_id/dataset/workspace_name/tag",
		)
		return
	}

	projectID := parts[0]
	dataset := parts[1]
	workspaceName := parts[2]
	tag := ""
	if len(parts) == 4 {
		tag = parts[3]
	}

	schemaID := schemaclient.SchemaID(workspaceName, tag)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dataset"), dataset)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_name"), workspaceName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), schemaID)...)
	if tag != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag"), tag)...)
	}
}

func (r *SchemaResource) updateModelFromDoc(data *SchemaResourceModel, doc *schemaclient.SchemaDocument) {
	data.ID = types.StringValue(doc.ID)
	data.WorkspaceName = types.StringValue(doc.Workspace.Name)
	data.Version = types.StringValue(doc.Version)

	if doc.Workspace.Title != "" {
		data.WorkspaceTitle = types.StringValue(doc.Workspace.Title)
	} else {
		data.WorkspaceTitle = types.StringValue("")
	}

	if doc.Tag != "" {
		data.Tag = types.StringValue(doc.Tag)
	}

	// Normalize the schema JSON from the API for consistent comparison
	schemaJSON, err := normalizeJSON(doc.Schema)
	if err != nil {
		data.Schema = jsontypes.NewNormalizedValue(string(doc.Schema))
	} else {
		data.Schema = jsontypes.NewNormalizedValue(schemaJSON)
	}
}

func normalizeJSON(raw json.RawMessage) (string, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}

	// The API may return the schema as a JSON string rather than a raw array.
	// If so, decode the string to get the actual array for normalization.
	if s, ok := v.(string); ok {
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return "", err
		}
	}

	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
