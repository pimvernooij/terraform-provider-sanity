package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tessellator/go-sanity/sanity"
)

var _ resource.Resource = &DatasetResource{}
var _ resource.ResourceWithImportState = &DatasetResource{}

func NewDatasetResource() resource.Resource {
	return &DatasetResource{}
}

type DatasetResource struct {
	client *sanity.Client
}

type DatasetResourceModel struct {
	Project types.String `tfsdk:"project"`
	Name    types.String `tfsdk:"name"`
	AclMode types.String `tfsdk:"acl_mode"`
}

func (r *DatasetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *DatasetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provides a dataset to a Sanity project. A dataset is like a database for your content, and you manage its contents with a studio and query it with GROQ or GraphQL.",

		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the project that the dataset belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"acl_mode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The ACL mode for the data. Valid options are `public` and `private`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *DatasetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *DatasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Projects.CreateDataset(ctx, data.Project.ValueString(), &sanity.CreateDatasetRequest{
		Name:    data.Name.ValueString(),
		AclMode: data.AclMode.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DatasetResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectId := data.Project.ValueString()

	datasets, err := r.client.Projects.ListDatasets(ctx, projectId)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	var dataset sanity.Dataset
	found := false

	for _, d := range datasets {
		if d.Name == data.Name.ValueString() {
			dataset = d
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("dataset not found", "dataset not found")
		return
	}

	data.AclMode = types.StringValue(dataset.AclMode)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Provider Error", "Update is not supported on dataset")
}

func (r *DatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DatasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Name.IsNull() {
		resp.Diagnostics.AddError("Name is null", "Name is null")
		return
	}
	if data.Project.IsNull() {
		resp.Diagnostics.AddError("Project is null", "Project is null")
		return
	}

	_, err := r.client.Projects.DeleteDataset(ctx, data.Project.ValueString(), data.Name.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("dataset %s could not be deleted, got error: %s", data.Name.ValueString(), err))
		return
	}
}

func (r *DatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Input Error", "The import identifier for a dataset should be in the form project-id/dataset-name")
		return
	}

	reqProject := resource.ImportStateRequest{ID: parts[0]}
	reqName := resource.ImportStateRequest{ID: parts[1]}

	resource.ImportStatePassthroughID(ctx, path.Root("project"), reqProject, resp)
	resource.ImportStatePassthroughID(ctx, path.Root("name"), reqName, resp)
}
