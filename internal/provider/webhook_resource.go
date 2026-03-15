package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	Type          types.String `tfsdk:"type"`
	Name          types.String `tfsdk:"name"`
	Dataset       types.String `tfsdk:"dataset"`
	URL           types.String `tfsdk:"url"`
	HttpMethod    types.String `tfsdk:"http_method"`
	ApiVersion    types.String `tfsdk:"api_version"`
	On            types.List   `tfsdk:"on"`
	Filter        types.String `tfsdk:"filter"`
	Projection    types.String `tfsdk:"projection"`
	IncludeDrafts types.Bool   `tfsdk:"include_drafts"`
	Headers       types.Map    `tfsdk:"headers"`
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
			"type": schema.StringAttribute{
				MarkdownDescription: "The webhook type. Must be `document` (GROQ-powered) or `transaction`. Defaults to `document`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("document"),
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The human-readable name for the webhook.",
				Required:            true,
			},
			"dataset": schema.StringAttribute{
				MarkdownDescription: "The dataset this webhook is configured for. Use `*` for all datasets.",
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
				MarkdownDescription: "The HTTP method used for webhook requests. One of `POST`, `PUT`, `PATCH`, `DELETE`, `GET`. Defaults to `POST`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("POST"),
			},
			"api_version": schema.StringAttribute{
				MarkdownDescription: "The API version used for the GROQ filter and projection. Defaults to `v2025-02-19`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("v2025-02-19"),
			},
			"on": schema.ListAttribute{
				MarkdownDescription: "The document events that trigger the webhook. Valid values: `create`, `update`, `delete`. Defaults to all three.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "A GROQ filter expression to determine which documents trigger the webhook.",
				Optional:            true,
			},
			"projection": schema.StringAttribute{
				MarkdownDescription: "A GROQ projection defining the webhook payload.",
				Optional:            true,
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
			"secret": schema.StringAttribute{
				MarkdownDescription: "Secret used for webhook signature verification. Not returned by the API after creation.",
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

	createReq := &sanity.CreateWebhookRequest{
		Type:    data.Type.ValueString(),
		Name:    data.Name.ValueString(),
		Dataset: data.Dataset.ValueString(),
		URL:     data.URL.ValueString(),
	}

	if !data.HttpMethod.IsNull() {
		createReq.HttpMethod = data.HttpMethod.ValueString()
	}
	if !data.ApiVersion.IsNull() && !data.ApiVersion.IsUnknown() {
		createReq.ApiVersion = data.ApiVersion.ValueString()
	}
	if !data.IncludeDrafts.IsNull() {
		createReq.IncludeDrafts = sanity.NewBool(data.IncludeDrafts.ValueBool())
	}

	// Headers
	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		headers := make(map[string]string)
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(headers) > 0 {
			createReq.Headers = headers
		}
	}

	// Build rule from on, filter, projection
	rule, diags := r.buildRule(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if rule != nil {
		createReq.Rule = rule
	}

	if !data.Secret.IsNull() {
		createReq.Secret = data.Secret.ValueString()
	}
	if !data.IsDisabled.IsNull() && data.IsDisabled.ValueBool() {
		createReq.IsDisabledByUser = sanity.NewBool(true)
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

	updateReq := &sanity.UpdateWebhookRequest{
		Name:       data.Name.ValueString(),
		URL:        data.URL.ValueString(),
		HttpMethod: data.HttpMethod.ValueString(),
	}

	if !data.ApiVersion.IsNull() && !data.ApiVersion.IsUnknown() {
		updateReq.ApiVersion = data.ApiVersion.ValueString()
	}
	if !data.IncludeDrafts.IsNull() {
		updateReq.IncludeDrafts = sanity.NewBool(data.IncludeDrafts.ValueBool())
	}

	// Headers
	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		headers := make(map[string]string)
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(headers) > 0 {
			updateReq.Headers = headers
		}
	}

	// Build rule from on, filter, projection
	rule, diags := r.buildRule(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if rule != nil {
		updateReq.Rule = rule
	}

	if !data.Secret.IsNull() {
		updateReq.Secret = data.Secret.ValueString()
	}
	if !data.IsDisabled.IsNull() {
		updateReq.IsDisabledByUser = sanity.NewBool(data.IsDisabled.ValueBool())
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
		// The Sanity API returns {"deleted": 1} (number) but go-sanity expects
		// a bool, causing a decode error. The delete succeeded if the error is
		// just a JSON unmarshal issue.
		if !strings.Contains(err.Error(), "cannot unmarshal") {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("webhook %s could not be deleted, got error: %s", data.Id.ValueString(), err))
			return
		}
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

// buildRule constructs the WebhookRule from on, filter, and projection attributes.
// Returns nil if no rule-related attributes are set.
func (r *WebhookResource) buildRule(ctx context.Context, data *WebhookResourceModel) (*sanity.WebhookRule, diag.Diagnostics) {
	var diags diag.Diagnostics

	hasFilter := !data.Filter.IsNull() && !data.Filter.IsUnknown()
	hasProjection := !data.Projection.IsNull() && !data.Projection.IsUnknown()
	hasOn := !data.On.IsNull() && !data.On.IsUnknown()

	if !hasFilter && !hasProjection && !hasOn {
		return nil, diags
	}

	rule := &sanity.WebhookRule{}

	if hasOn {
		var on []string
		diags.Append(data.On.ElementsAs(ctx, &on, false)...)
		rule.On = on
	} else {
		// Default to all events when rule is present but on is not specified
		rule.On = []string{"create", "update", "delete"}
	}

	if hasFilter {
		rule.Filter = data.Filter.ValueString()
	}
	if hasProjection {
		rule.Projection = data.Projection.ValueString()
	}

	return rule, diags
}

// updateModelFromWebhook updates the Terraform model with data from the API webhook struct.
func (r *WebhookResource) updateModelFromWebhook(ctx context.Context, data *WebhookResourceModel, webhook *sanity.Webhook) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringValue(webhook.Id)
	data.ProjectId = types.StringValue(webhook.ProjectId)
	data.Type = types.StringValue(webhook.Type)
	data.Name = types.StringValue(webhook.Name)
	data.Dataset = types.StringValue(webhook.Dataset)
	data.URL = types.StringValue(webhook.URL)
	data.HttpMethod = types.StringValue(webhook.HttpMethod)
	data.ApiVersion = types.StringValue(webhook.ApiVersion)
	data.IncludeDrafts = types.BoolValue(webhook.IncludeDrafts)
	data.IsDisabled = types.BoolValue(webhook.IsDisabled)

	if !webhook.CreatedAt.IsZero() {
		data.CreatedAt = types.StringValue(webhook.CreatedAt.Format("2006-01-02T15:04:05Z"))
	}
	if !webhook.UpdatedAt.IsZero() {
		data.UpdatedAt = types.StringValue(webhook.UpdatedAt.Format("2006-01-02T15:04:05Z"))
	} else {
		data.UpdatedAt = types.StringNull()
	}

	// Rule → on, filter, projection
	if webhook.Rule != nil {
		if len(webhook.Rule.On) > 0 {
			elems := make([]attr.Value, len(webhook.Rule.On))
			for i, v := range webhook.Rule.On {
				elems[i] = types.StringValue(v)
			}
			data.On = types.ListValueMust(types.StringType, elems)
		} else {
			data.On = types.ListNull(types.StringType)
		}

		if webhook.Rule.Filter != "" {
			data.Filter = types.StringValue(webhook.Rule.Filter)
		} else {
			data.Filter = types.StringNull()
		}

		if webhook.Rule.Projection != "" {
			data.Projection = types.StringValue(webhook.Rule.Projection)
		} else {
			data.Projection = types.StringNull()
		}
	} else {
		data.On = types.ListNull(types.StringType)
		data.Filter = types.StringNull()
		data.Projection = types.StringNull()
	}

	// Headers
	if webhook.Headers != nil && len(webhook.Headers) > 0 {
		mapVal, d := types.MapValueFrom(ctx, types.StringType, webhook.Headers)
		diags.Append(d...)
		data.Headers = mapVal
	} else {
		data.Headers = types.MapNull(types.StringType)
	}

	// Secret is write-only; preserve existing state value
	// (API does not return the secret after creation)

	return diags
}
