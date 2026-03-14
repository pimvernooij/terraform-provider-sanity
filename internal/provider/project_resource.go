package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tessellator/go-sanity/sanity"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

type ProjectResource struct {
	client *sanity.Client
}

// ProjectResourceModel describes the resource data model.
type ProjectResourceModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Organization        types.String `tfsdk:"organization"`
	StudioHost          types.String `tfsdk:"studio_host"`
	ExternalStudioHost  types.String `tfsdk:"external_studio_host"`
	Color               types.String `tfsdk:"color"`
	IsDisabledByUser    types.Bool   `tfsdk:"disabled_by_user"`
	ActivityFeedEnabled types.Bool   `tfsdk:"activity_feed_enabled"`
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provides a Sanity project. A project is the base resource for creating content, and the project may contain datasets, CORS origins, and tags.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The project ID, which you can find at the top of the project page in Sanity.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The project name.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				MarkdownDescription: "The name of the organization that owns the project.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"studio_host": schema.StringAttribute{
				MarkdownDescription: "The studio host URL. This attribute exhibits two unique behaviors that are important to note. First, once the studio host URL is set, it may not be changed. Changing this value will force a replacement. Second, when the studio host is set, Sanity will automatically create a CORS entry for the studio host URL. This means that it is not necessary for you to create a CORS entry, and you will get a conflict error if you do.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"external_studio_host": schema.StringAttribute{
				MarkdownDescription: "The external studio host URL.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "The hex value for the project color. This is the color of the project icon at https://sanity.io/manage.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"disabled_by_user": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether the project is archived. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"activity_feed_enabled": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether the [activity feed](https://www.sanity.io/docs/activity-feed) is enabled. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

	r.client = clients.SanityClient
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.Projects.Create(ctx, &sanity.CreateProjectRequest{
		DisplayName:    data.Name.ValueString(),
		OrganizationId: data.Organization.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	entries, err := r.client.Projects.ListCORSEntries(ctx, project.Id)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}
	for _, entry := range entries {
		_, err = r.client.Projects.DeleteCORSEntry(ctx, project.Id, entry.Id)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			r.client.Projects.Delete(ctx, project.Id)
			return
		}
	}

	requiresUpdate := !data.StudioHost.IsNull() ||
		!data.ExternalStudioHost.IsNull() ||
		!data.Color.IsNull() ||
		!data.IsDisabledByUser.IsNull() ||
		!data.ActivityFeedEnabled.IsNull()

	if requiresUpdate {
		updateReq := &sanity.UpdateProjectRequest{}
		if !data.StudioHost.IsNull() {
			updateReq.StudioHost = data.StudioHost.ValueString()
		}
		if !data.ExternalStudioHost.IsNull() {
			updateReq.ExternalStudioHost = data.ExternalStudioHost.ValueString()
		}
		if !data.Color.IsNull() {
			updateReq.Color = data.Color.ValueString()
		}
		if !data.IsDisabledByUser.IsNull() {
			updateReq.IsDisabledByUser = sanity.NewBool(data.IsDisabledByUser.ValueBool())
		}
		if !data.ActivityFeedEnabled.IsNull() {
			updateReq.ActivityFeedEnabled = sanity.NewBool(data.ActivityFeedEnabled.ValueBool())
		}
		project, err = r.client.Projects.Update(ctx, project.Id, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			r.client.Projects.Delete(ctx, project.Id)
			return
		}
	}

	data.Id = types.StringValue(project.Id)
	data.Name = types.StringValue(project.DisplayName)
	data.Organization = types.StringValue(project.OrganizationId)
	data.StudioHost = types.StringValue(project.StudioHost)
	data.ExternalStudioHost = types.StringValue(project.Metadata["externalStudioHost"])
	data.Color = types.StringValue(project.Metadata["color"])
	data.IsDisabledByUser = types.BoolValue(project.IsDisabledByUser)
	data.ActivityFeedEnabled = types.BoolValue(project.ActivityFeedEnabled)

	tflog.Trace(ctx, "created a sanity project", map[string]interface{}{"id": project.Id, "name": project.DisplayName})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ProjectResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		resp.Diagnostics.AddError("Project id is null", "Project id is null")
		return
	}

	project, err := r.client.Projects.Get(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	data.Id = types.StringValue(project.Id)
	data.Name = types.StringValue(project.DisplayName)
	data.Organization = types.StringValue(project.OrganizationId)
	data.StudioHost = types.StringValue(project.StudioHost)
	data.ExternalStudioHost = types.StringValue(project.Metadata["externalStudioHost"])
	data.Color = types.StringValue(project.Metadata["color"])
	data.IsDisabledByUser = types.BoolValue(project.IsDisabledByUser)
	data.ActivityFeedEnabled = types.BoolValue(project.ActivityFeedEnabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		resp.Diagnostics.AddError("Project id is null", "Project id is null")
		return
	}

	var studioHost string
	req.State.GetAttribute(ctx, path.Root("studio_host"), &studioHost)

	requiresUpdate := !data.Name.IsNull() ||
		(!data.StudioHost.IsNull() && studioHost == "") ||
		!data.ExternalStudioHost.IsNull() ||
		!data.Color.IsNull() ||
		!data.IsDisabledByUser.IsNull() ||
		!data.ActivityFeedEnabled.IsNull()

	if !requiresUpdate {
		return
	}

	updateReq := &sanity.UpdateProjectRequest{}
	if !data.Name.IsNull() {
		updateReq.DisplayName = data.Name.ValueString()
	}
	if studioHost == "" && !data.StudioHost.IsNull() {
		updateReq.StudioHost = data.StudioHost.ValueString()
	}
	if !data.ExternalStudioHost.IsNull() {
		updateReq.ExternalStudioHost = data.ExternalStudioHost.ValueString()
	}
	if !data.Color.IsNull() {
		updateReq.Color = data.Color.ValueString()
	}
	if !data.IsDisabledByUser.IsNull() {
		updateReq.IsDisabledByUser = sanity.NewBool(data.IsDisabledByUser.ValueBool())
	}
	if !data.ActivityFeedEnabled.IsNull() {
		updateReq.ActivityFeedEnabled = sanity.NewBool(data.ActivityFeedEnabled.ValueBool())
	}
	project, err := r.client.Projects.Update(ctx, data.Id.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		r.client.Projects.Delete(ctx, project.Id)
		return
	}

	data.Id = types.StringValue(project.Id)
	data.Name = types.StringValue(project.DisplayName)
	data.Organization = types.StringValue(project.OrganizationId)
	data.StudioHost = types.StringValue(project.StudioHost)
	data.ExternalStudioHost = types.StringValue(project.Metadata["externalStudioHost"])
	data.Color = types.StringValue(project.Metadata["color"])
	data.IsDisabledByUser = types.BoolValue(project.IsDisabledByUser)
	data.ActivityFeedEnabled = types.BoolValue(project.ActivityFeedEnabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		resp.Diagnostics.AddError("Project id is null", "Project id is null")
		return
	}

	_, err := r.client.Projects.Delete(ctx, data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("project %s could not be deleted, got error: %s", data.Id.ValueString(), err))
		return
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
