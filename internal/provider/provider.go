package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tessellator/go-sanity/sanity"
	"github.com/tessellator/terraform-provider-sanity/internal/schemaclient"
	"github.com/tessellator/terraform-provider-sanity/internal/studioclient"
	"golang.org/x/oauth2"
)

// ProviderClients wraps both the go-sanity client and the schema API client
// so that all resources can access their respective clients.
type ProviderClients struct {
	SanityClient *sanity.Client
	SchemaClient *schemaclient.Client
	StudioClient *studioclient.Client
}

var _ provider.Provider = &SanityProvider{}

// SanityProvider defines the provider implementation.
type SanityProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and run locally, and "test" when running acceptance
	// testing.
	version string
}

// SanityProviderModel describes the provider data model.
type SanityProviderModel struct {
	Token types.String `tfsdk:"token"`
}

func (p *SanityProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "sanity"
	resp.Version = p.version
}

func (p *SanityProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				MarkdownDescription: "The auth token used to authenticate with Sanity. May be sourced from the `SANITY_TOKEN` environment variable instead of via this attribute.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *SanityProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config SanityProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var token string
	if config.Token.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create client",
			"Cannot use unknown value as token",
		)
		return
	}

	if config.Token.IsNull() {
		token = os.Getenv("SANITY_TOKEN")
	} else {
		token = config.Token.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Unable to find token",
			"Token cannot be an empty string",
		)
		return
	}

	tokenSrc := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(context.Background(), tokenSrc)

	clients := &ProviderClients{
		SanityClient: sanity.NewClient(httpClient),
		SchemaClient: schemaclient.NewClient(httpClient),
		StudioClient: studioclient.NewClient(httpClient),
	}
	resp.DataSourceData = clients
	resp.ResourceData = clients
}

func (p *SanityProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewCORSOriginResource,
		NewDatasetResource,
		NewProjectTokenResource,
		NewWebhookResource,
		NewSchemaResource,
		NewSchemaTypeResource,
		NewStudioDeploymentResource,
	}
}

func (p *SanityProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewSchemaDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SanityProvider{
			version: version,
		}
	}
}
