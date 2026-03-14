package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tessellator/terraform-provider-sanity/internal/schemaclient"
)

var _ datasource.DataSource = &SchemaDataSource{}

func NewSchemaDataSource() datasource.DataSource {
	return &SchemaDataSource{}
}

type SchemaDataSource struct {
	client *schemaclient.Client
}

type SchemaDataSourceModel struct {
	ID             types.String         `tfsdk:"id"`
	ProjectID      types.String         `tfsdk:"project_id"`
	Dataset        types.String         `tfsdk:"dataset"`
	WorkspaceName  types.String         `tfsdk:"workspace_name"`
	WorkspaceTitle types.String         `tfsdk:"workspace_title"`
	Version        types.String         `tfsdk:"version"`
	Tag            types.String         `tfsdk:"tag"`
	Schema         jsontypes.Normalized `tfsdk:"schema"`
}

func (d *SchemaDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (d *SchemaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Sanity content schema for a workspace. Use this data source to reference an existing deployed schema.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The schema document ID.",
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The project ID.",
			},
			"dataset": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The dataset name.",
			},
			"workspace_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The workspace name, e.g. `default`.",
			},
			"workspace_title": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The workspace title.",
			},
			"version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The schema version.",
			},
			"tag": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "An optional tag for the schema.",
			},
			"schema": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The schema definition as a JSON array of type objects.",
				CustomType:          jsontypes.NormalizedType{},
			},
		},
	}
}

func (d *SchemaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = clients.SchemaClient
}

func (d *SchemaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SchemaDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	schemaID := schemaclient.SchemaID(data.WorkspaceName.ValueString(), data.Tag.ValueString())

	doc, err := d.client.GetSchema(ctx, data.ProjectID.ValueString(), data.Dataset.ValueString(), schemaID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read schema: %s", err))
		return
	}

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
	} else {
		data.Tag = types.StringValue("")
	}

	schemaJSON, err := normalizeJSON(doc.Schema)
	if err != nil {
		data.Schema = jsontypes.NewNormalizedValue(string(doc.Schema))
	} else {
		data.Schema = jsontypes.NewNormalizedValue(schemaJSON)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
