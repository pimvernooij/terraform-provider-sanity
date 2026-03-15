package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tessellator/go-sanity/sanity"
)

var _ resource.Resource = &WebhookResource{}
var _ resource.ResourceWithImportState = &WebhookResource{}

func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

type WebhookResource struct {
	client *sanity.Client
}

// WebhookResourceModel describes the resource data model.
type WebhookResourceModel struct {
	Id            types.String `tfsdk:"id"`
	ProjectId     types.String `tfsdk:"project_id"`
	Name          types.String `tfsdk:"name"`
	Dataset       types.String `tfsdk:"dataset"`
	URL           types.String `tfsdk:"url"`
	HttpMethod    types.String `tfsdk:"http_method"`
	ApiVersion    types.String `tfsdk:"api_version"`
	IncludeDrafts types.Bool   `tfsdk:"include_drafts"`
	Headers       types.Map    `tfsdk:"headers"`
	Filter        types.String `tfsdk:"filter"`
	Secret        types.String `tfsdk:"secret"`
	IsDisabled    types.Bool   `tfsdk:"is_disabled"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *WebhookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provides a Sanity webhook. Webhooks allow you to get notified when content is created, updated, or deleted in your Sanity project.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The webhook ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The project ID that this webhook belongs to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The human-readable name for the webhook.",
				Required:            true,
			},
			"dataset": schema.StringAttribute{
				MarkdownDescription: "The dataset this webhook is configured for.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The endpoint URL that will receive webhook notifications.",
				Required:            true,
			},
			"http_method": schema.StringAttribute{
				MarkdownDescription: "The HTTP method used for webhook requests. Defaults to `POST`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("POST"),
			},
			"api_version": schema.StringAttribute{
				MarkdownDescription: "The API version used for webhook payloads. Defaults to the current API version.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"include_drafts": schema.BoolAttribute{
				MarkdownDescription: "Whether draft documents trigger webhook notifications. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"headers": schema.MapAttribute{
				MarkdownDescription: "Custom HTTP headers sent with webhook requests.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "A GROQ filter expression to determine which documents trigger the webhook.",
				Optional:            true,
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "Secret used for webhook signature verification.",
				Optional:            true,
				Sensitive:           true,
			},
			"is_disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the webhook is currently disabled. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The time the webhook was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The time the webhook was last updated.",
			},
		},
	}
}

func (r *WebhookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert headers map from Terraform types to Go strings
	headers := make(map[string]string)
	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	createReq := &sanity.CreateWebhookRequest{
		Name:    data.Name.ValueString(),
		Dataset: data.Dataset.ValueString(),
		URL:     data.URL.ValueString(),
	}

	if !data.HttpMethod.IsNull() {
		createReq.HttpMethod = data.HttpMethod.ValueString()
	}
	if !data.ApiVersion.IsNull() {
		createReq.ApiVersion = data.ApiVersion.ValueString()
	}
	if !data.IncludeDrafts.IsNull() {
		createReq.IncludeDrafts = sanity.NewBool(data.IncludeDrafts.ValueBool())
	}
	if len(headers) > 0 {
		createReq.Headers = headers
	}
	if !data.Filter.IsNull() {
		if createReq.Rule == nil {
			createReq.Rule = &sanity.WebhookRule{}
		}
		createReq.Rule.Filter = data.Filter.ValueString()
	}
	if !data.Secret.IsNull() {
		createReq.Secret = data.Secret.ValueString()
	}

	webhook, err := r.client.Webhooks.Create(ctx, data.ProjectId.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	resp.Diagnostics.Append(r.updateModelFromWebhook(ctx, data, webhook)...)

	tflog.Trace(ctx, "created a sanity webhook", map[string]interface{}{"id": webhook.Id, "project_id": webhook.ProjectId})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *WebhookResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.ProjectId.IsNull() {
		resp.Diagnostics.AddError("Missing required values", "Webhook id and project_id are required")
		return
	}

	webhook, err := r.client.Webhooks.Get(ctx, data.ProjectId.ValueString(), data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	resp.Diagnostics.Append(r.updateModelFromWebhook(ctx, data, webhook)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.ProjectId.IsNull() {
		resp.Diagnostics.AddError("Missing required values", "Webhook id and project_id are required")
		return
	}

	updateReq := &sanity.UpdateWebhookRequest{}
	requiresUpdate := false

	if !data.Name.IsNull() {
		updateReq.Name = data.Name.ValueString()
		requiresUpdate = true
	}
	if !data.URL.IsNull() {
		updateReq.URL = data.URL.ValueString()
		requiresUpdate = true
	}
	if !data.HttpMethod.IsNull() {
		updateReq.HttpMethod = data.HttpMethod.ValueString()
		requiresUpdate = true
	}
	if !data.ApiVersion.IsNull() {
		updateReq.ApiVersion = data.ApiVersion.ValueString()
		requiresUpdate = true
	}
	if !data.IncludeDrafts.IsNull() {
		updateReq.IncludeDrafts = sanity.NewBool(data.IncludeDrafts.ValueBool())
		requiresUpdate = true
	}
	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		headers := make(map[string]string)
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(headers) > 0 {
			updateReq.Headers = headers
			requiresUpdate = true
		}
	}
	if !data.Filter.IsNull() {
		if updateReq.Rule == nil {
			updateReq.Rule = &sanity.WebhookRule{}
		}
		updateReq.Rule.Filter = data.Filter.ValueString()
		requiresUpdate = true
	}
	if !data.Secret.IsNull() {
		updateReq.Secret = data.Secret.ValueString()
		requiresUpdate = true
	}
	if !data.IsDisabled.IsNull() {
		updateReq.IsDisabledByUser = sanity.NewBool(data.IsDisabled.ValueBool())
		requiresUpdate = true
	}

	if !requiresUpdate {
		return
	}

	webhook, err := r.client.Webhooks.Update(ctx, data.ProjectId.ValueString(), data.Id.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	resp.Diagnostics.Append(r.updateModelFromWebhook(ctx, data, webhook)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.ProjectId.IsNull() {
		resp.Diagnostics.AddError("Missing required values", "Webhook id and project_id are required")
		return
	}

	_, err := r.client.Webhooks.Delete(ctx, data.ProjectId.ValueString(), data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("webhook %s could not be deleted, got error: %s", data.Id.ValueString(), err))
		return
	}
}

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Input Error", "The import identifier for a webhook should be in the form project-id/webhook-id")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// updateModelFromWebhook updates the Terraform model with data from the API webhook struct
func (r *WebhookResource) updateModelFromWebhook(ctx context.Context, data *WebhookResourceModel, webhook *sanity.Webhook) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringValue(webhook.Id)
	data.ProjectId = types.StringValue(webhook.ProjectId)
	data.Name = types.StringValue(webhook.Name)
	data.Dataset = types.StringValue(webhook.Dataset)
	data.URL = types.StringValue(webhook.URL)
	data.HttpMethod = types.StringValue(webhook.HttpMethod)
	data.ApiVersion = types.StringValue(webhook.ApiVersion)
	data.IncludeDrafts = types.BoolValue(webhook.IncludeDrafts)
	data.IsDisabled = types.BoolValue(webhook.IsDisabled)
	data.CreatedAt = types.StringValue(webhook.CreatedAt.Format("2006-01-02T15:04:05Z"))
	data.UpdatedAt = types.StringValue(webhook.UpdatedAt.Format("2006-01-02T15:04:05Z"))

	// Set filter from Rule - use null for empty/absent filter since it's an optional field
	if webhook.Rule != nil && webhook.Rule.Filter != "" {
		data.Filter = types.StringValue(webhook.Rule.Filter)
	} else {
		data.Filter = types.StringNull()
	}

	// Convert headers map from Go strings to Terraform types
	if webhook.Headers != nil && len(webhook.Headers) > 0 {
		mapVal, d := types.MapValueFrom(ctx, types.StringType, webhook.Headers)
		diags.Append(d...)
		data.Headers = mapVal
	} else {
		data.Headers = types.MapNull(types.StringType)
	}

	// Preserve the secret from state since it's not returned by the API
	if webhook.Secret != "" {
		data.Secret = types.StringValue(webhook.Secret)
	}

	return diags
}
