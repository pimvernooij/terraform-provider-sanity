package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tessellator/go-sanity/sanity"
)

var _ resource.Resource = &CORSOriginResource{}
var _ resource.ResourceWithImportState = &CORSOriginResource{}

func NewCORSOriginResource() resource.Resource {
	return &CORSOriginResource{}
}

type CORSOriginResource struct {
	client *sanity.Client
}

type CORSOriginResourceModel struct {
	Id               types.String `tfsdk:"id"`
	Origin           types.String `tfsdk:"origin"`
	AllowCredentials types.Bool   `tfsdk:"allow_credentials"`
	Project          types.String `tfsdk:"project"`
}

func (r *CORSOriginResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cors_origin"
}

func (r *CORSOriginResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provides a CORS origin to a Sanity project. A CORS origin is a host that can connect to the Sanity Project API.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID for the CORS origin.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"origin": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The origin you want to allow traffic from, stating explicitly the protocol, host name and port. Wildcards (`*`) are allowed. Use the following format: `protocol://host:port`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allow_credentials": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Indicates whether the origin is allowed to send credentials (e.g. a session cookie or an authorization token). Defaults to `true`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the project that the CORS origin belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *CORSOriginResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CORSOriginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *CORSOriginResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	corsReq := &sanity.CreateCORSEntryRequest{
		Origin:           data.Origin.ValueString(),
		AllowCredentials: sanity.NewBool(true),
	}
	if !data.AllowCredentials.IsNull() {
		corsReq.AllowCredentials = sanity.NewBool(data.AllowCredentials.ValueBool())
	}
	entry, err := r.client.Projects.CreateCORSEntry(ctx, data.Project.ValueString(), corsReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%d", entry.Id))
	data.AllowCredentials = types.BoolValue(entry.AllowCredentials)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CORSOriginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *CORSOriginResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		resp.Diagnostics.AddError("Entry id is null", "Entry id is null")
		return
	}
	if data.Project.IsNull() {
		resp.Diagnostics.AddError("Project is null", "Project is null")
		return
	}

	entries, err := r.client.Projects.ListCORSEntries(ctx, data.Project.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	rawId := int64(0)
	_, err = fmt.Sscanf(data.Id.ValueString(), "%d", &rawId)
	if err != nil {
		resp.Diagnostics.AddError("Invalid CORS Entry ID",
			fmt.Sprintf("Could not parse CORS entry ID %q as integer: %s", data.Id.ValueString(), err))
		return
	}

	var entry sanity.CORSEntry
	found := false
	for _, e := range entries {
		if e.Id == rawId {
			entry = e
			found = true
			break
		}
	}
	if !found {
		resp.Diagnostics.AddError("CORS Entry Not Found",
			fmt.Sprintf("CORS entry %s not found in project %s", data.Id.ValueString(), data.Project.ValueString()))
		return
	}

	data.AllowCredentials = types.BoolValue(entry.AllowCredentials)
	data.Origin = types.StringValue(entry.Origin)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CORSOriginResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Provider Error", "Update is not supported on CORS entry")
}

func (r *CORSOriginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *CORSOriginResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		resp.Diagnostics.AddError("Entry id is null", "Entry id is null")
		return
	}
	if data.Project.IsNull() {
		resp.Diagnostics.AddError("Project is null", "Project is null")
		return
	}

	rawId := int64(0)
	_, err := fmt.Sscanf(data.Id.ValueString(), "%d", &rawId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	_, err = r.client.Projects.DeleteCORSEntry(ctx, data.Project.ValueString(), rawId)

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("entry %s could not be deleted, got error: %s", data.Id.ValueString(), err))
		return
	}
}

func (r *CORSOriginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectId, origin, _ := strings.Cut(req.ID, "/")
	if projectId == "" || origin == "" {
		resp.Diagnostics.AddError("Import Error", "The format for importing a CORS origin is project-id/origin")
		return
	}

	entries, err := r.client.Projects.ListCORSEntries(ctx, projectId)
	if err != nil {
		resp.Diagnostics.AddError("Import Error", err.Error())
		return
	}

	for _, e := range entries {
		if e.Origin == origin {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fmt.Sprintf("%d", e.Id))...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), projectId)...)
			return
		}
	}

	resp.Diagnostics.AddError("Import Error", "The requested CORS origin was not found")
}
