// Package provider is the Terraform Plugin Framework provider for
// aptabase-plus. It wraps the standalone src/aptabase client; it does
// not talk to aptabase-plus directly itself.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

var _ provider.Provider = &AptabaseProvider{}

// New returns a provider.Provider factory, as required by
// providerserver.Serve. version is injected at build time (see src/main.go).
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AptabaseProvider{version: version}
	}
}

type AptabaseProvider struct {
	version string
}

type aptabaseProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

func (p *AptabaseProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "aptabase"
	resp.Version = p.version
}

func (p *AptabaseProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages apps, app shares, and API keys on a self-hosted " +
			"[aptabase-plus](https://github.com/nmehlei/aptabase-plus) instance.\n\n" +
			"~> This provider does **not** work against Aptabase Cloud or stock self-hosted " +
			"Aptabase - only against `aptabase-plus`, which adds the API-key-authenticated " +
			"management API this provider depends on.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the aptabase-plus instance, e.g. `https://aptabase.example.com`.",
				Required:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "API key for the aptabase-plus management API. Falls back to the " +
					"`APTABASE_TOKEN` environment variable when unset; configuration fails if neither is present.",
				Optional:  true,
				Sensitive: true,
			},
			"insecure": schema.BoolAttribute{
				MarkdownDescription: "Allow a plain-HTTP endpoint. Only for local development against a " +
					"disposable instance - never set this against a production endpoint.",
				Optional: true,
			},
		},
	}
}

func (p *AptabaseProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data aptabaseProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Endpoint.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing Aptabase Endpoint",
			"The endpoint attribute is required, e.g. https://aptabase.example.com.",
		)
		return
	}

	token := data.Token.ValueString()
	if token == "" {
		token = os.Getenv("APTABASE_TOKEN")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Aptabase API Token",
			"Set the token attribute in the provider configuration, or the APTABASE_TOKEN environment variable.",
		)
		return
	}

	client, err := aptabase.New(aptabase.Config{
		Endpoint:      data.Endpoint.ValueString(),
		Token:         token,
		AllowInsecure: data.Insecure.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure Aptabase Client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *AptabaseProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAppResource,
		NewAppShareResource,
		NewApiKeyResource,
	}
}

func (p *AptabaseProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAppDataSource,
		NewAppsDataSource,
	}
}

