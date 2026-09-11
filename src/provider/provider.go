// Package provider is the Terraform Plugin Framework provider for
// aptabase-plus. It wraps the standalone src/aptabase client; it does
// not talk to aptabase-plus directly itself.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

// Stub resources and data sources below are placeholders for Tasks 6-9.
//
// They cannot simply panic() in their constructor or in Metadata/Schema:
// terraform-plugin-framework's GetProviderSchema handler unconditionally
// instantiates every registered resource/data source and calls Metadata and
// Schema on it to assemble the full provider schema (see
// fwserver.Server.ResourceFuncs / ResourceSchemas), so a panic there breaks
// schema discovery for the whole provider - including Task 4's own
// TestProvider_Metadata_ReportsAptabaseTypeName test. Panicking is therefore
// deferred to the CRUD/Read methods, which are only invoked once Terraform
// actually operates on the resource/data source - not implemented until the
// task named in each panic message.
//
// Each task that implements one of these for real (app_resource.go etc.)
// deletes the corresponding stub and its constructor below.

type stubApiKeyResource struct{ notImplementedUntil string }

func (r *stubApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}
func (r *stubApiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{}
}
func (r *stubApiKeyResource) Create(_ context.Context, _ resource.CreateRequest, _ *resource.CreateResponse) {
	panic(r.notImplementedUntil)
}
func (r *stubApiKeyResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	panic(r.notImplementedUntil)
}
func (r *stubApiKeyResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	panic(r.notImplementedUntil)
}
func (r *stubApiKeyResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	panic(r.notImplementedUntil)
}

func NewApiKeyResource() resource.Resource {
	return &stubApiKeyResource{notImplementedUntil: "implemented in Task 8"}
}

type stubAppDataSource struct{ notImplementedUntil string }

func (d *stubAppDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}
func (d *stubAppDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{}
}
func (d *stubAppDataSource) Read(_ context.Context, _ datasource.ReadRequest, _ *datasource.ReadResponse) {
	panic(d.notImplementedUntil)
}

func NewAppDataSource() datasource.DataSource {
	return &stubAppDataSource{notImplementedUntil: "implemented in Task 9"}
}

type stubAppsDataSource struct{ notImplementedUntil string }

func (d *stubAppsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apps"
}
func (d *stubAppsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{}
}
func (d *stubAppsDataSource) Read(_ context.Context, _ datasource.ReadRequest, _ *datasource.ReadResponse) {
	panic(d.notImplementedUntil)
}

func NewAppsDataSource() datasource.DataSource {
	return &stubAppsDataSource{notImplementedUntil: "implemented in Task 9"}
}
