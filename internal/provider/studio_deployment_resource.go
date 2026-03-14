package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tessellator/terraform-provider-sanity/internal/studioclient"
)

var _ resource.Resource = &StudioDeploymentResource{}
var _ resource.ResourceWithImportState = &StudioDeploymentResource{}

func NewStudioDeploymentResource() resource.Resource {
	return &StudioDeploymentResource{}
}

type StudioDeploymentResource struct {
	client *studioclient.Client
}

type StudioDeploymentResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	Hostname       types.String `tfsdk:"hostname"`
	Bundle         types.String `tfsdk:"bundle"`
	SourceCodeHash types.String `tfsdk:"source_code_hash"`
	AutoUpdates    types.Bool   `tfsdk:"auto_updates"`
	Version        types.String `tfsdk:"version"`
	URL            types.String `tfsdk:"url"`
}

func (r *StudioDeploymentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_studio_deployment"
}

func (r *StudioDeploymentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deploys a pre-built Sanity Studio bundle to Sanity's hosted studio infrastructure. The studio will be available at `<hostname>.sanity.studio`. Build your studio with `sanity build` and provide the resulting tarball.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The user application ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The project ID to deploy the studio for.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The hostname for the deployed studio. The studio will be available at `<hostname>.sanity.studio`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bundle": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Path to the `.tar.gz` bundle of the built studio. Build with `sanity build` then `tar -czf studio.tar.gz -C dist .`.",
			},
			"source_code_hash": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Hash of the bundle file used to trigger redeployment. Use `filebase64sha256()` on the bundle path.",
			},
			"auto_updates": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the studio should automatically update to the latest version. Defaults to `false`.",
			},
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The Sanity package version string, e.g. `3.0.0`. Informational metadata sent with the deployment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The URL of the deployed studio, e.g. `https://my-company.sanity.studio`.",
			},
		},
	}
}

func (r *StudioDeploymentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clients.StudioClient
}

func (r *StudioDeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data StudioDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	hostname := data.Hostname.ValueString()

	// Find or create the user application
	app, err := r.client.FindApplication(ctx, projectID, hostname)
	if err != nil {
		// Not found, create it
		app, err = r.client.CreateApplication(ctx, projectID, hostname)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create studio application: %s", err))
			return
		}
	}

	// Deploy the bundle
	deployResp, err := r.client.CreateDeployment(ctx, projectID, app.ID, &studioclient.DeployOptions{
		BundlePath:  data.Bundle.ValueString(),
		AutoUpdates: data.AutoUpdates.ValueBool(),
		Version:     data.Version.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to deploy studio bundle: %s", err))
		return
	}

	data.ID = types.StringValue(app.ID)
	data.URL = types.StringValue(deployResp.Location)
	if data.Version.IsNull() || data.Version.IsUnknown() {
		data.Version = types.StringValue("")
	}

	tflog.Trace(ctx, "deployed sanity studio", map[string]interface{}{"id": app.ID, "url": deployResp.Location})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StudioDeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data StudioDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApplication(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read studio application: %s", err))
		return
	}

	data.Hostname = types.StringValue(app.AppHost)
	if app.URL != "" {
		data.URL = types.StringValue(app.URL)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StudioDeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data StudioDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-deploy with the new bundle
	deployResp, err := r.client.CreateDeployment(ctx, data.ProjectID.ValueString(), data.ID.ValueString(), &studioclient.DeployOptions{
		BundlePath:  data.Bundle.ValueString(),
		AutoUpdates: data.AutoUpdates.ValueBool(),
		Version:     data.Version.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to deploy studio bundle: %s", err))
		return
	}

	data.URL = types.StringValue(deployResp.Location)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StudioDeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data StudioDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteApplication(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete studio deployment: %s", err))
		return
	}
}

func (r *StudioDeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: project_id/hostname
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Import Error",
			"The import identifier for a studio deployment should be in the form project_id/hostname",
		)
		return
	}

	projectID := parts[0]
	hostname := parts[1]

	app, err := r.client.FindApplication(ctx, projectID, hostname)
	if err != nil {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Unable to find studio application: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), app.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("hostname"), app.AppHost)...)
	if app.URL != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("url"), app.URL)...)
	}
}
