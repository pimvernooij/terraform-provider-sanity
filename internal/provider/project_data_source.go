package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tessellator/go-sanity/sanity"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ datasource.DataSource = &ProjectDataSource{}

func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

// ProjectDataSource defines the data source implementation.
type ProjectDataSource struct {
	client *sanity.Client
}

// ProjectDataSourceModel describes the data source data model.
type ProjectDataSourceModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Organization        types.String `tfsdk:"organization"`
	StudioHost          types.String `tfsdk:"studio_host"`
	ExternalStudioHost  types.String `tfsdk:"external_studio_host"`
	IsDisabledByUser    types.Bool   `tfsdk:"disabled_by_user"`
	ActivityFeedEnabled types.Bool   `tfsdk:"activity_feed_enabled"`
}

func (d *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gets a Sanity project by its ID. A project is the base resource for creating content, and the project may contain datasets, CORS origins, and tags.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The project ID, which you can find at the top of the project page in Sanity.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The project name.",
				Computed:            true,
			},
			"organization": schema.StringAttribute{
				MarkdownDescription: "The name of the organization that owns the project.",
				Computed:            true,
			},
			"studio_host": schema.StringAttribute{
				MarkdownDescription: "The studio host URL.",
				Computed:            true,
			},
			"external_studio_host": schema.StringAttribute{
				MarkdownDescription: "The external studio host URL.",
				Computed:            true,
			},
			"disabled_by_user": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether the project is archived.",
				Computed:            true,
			},
			"activity_feed_enabled": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether the [activity feed](https://www.sanity.io/docs/activity-feed) is enabled.",
				Computed:            true,
			},
		},
	}
}

func (d *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

	d.client = clients.SanityClient
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		resp.Diagnostics.AddError("Project id is null", "Project id is null")
		return
	}

	project, err := d.client.Projects.Get(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	data.Id = types.StringValue(project.Id)
	data.Name = types.StringValue(project.DisplayName)
	data.Organization = types.StringValue(project.OrganizationId)
	data.StudioHost = types.StringValue(project.StudioHost)
	data.ExternalStudioHost = types.StringValue(project.Metadata["externalStudioHost"])
	data.IsDisabledByUser = types.BoolValue(project.IsDisabledByUser)
	data.ActivityFeedEnabled = types.BoolValue(project.ActivityFeedEnabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
