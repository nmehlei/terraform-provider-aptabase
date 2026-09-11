package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

var _ datasource.DataSource = &AppsDataSource{}

func NewAppsDataSource() datasource.DataSource {
	return &AppsDataSource{}
}

type AppsDataSource struct {
	client *aptabase.Client
}

type appsDataSourceModel struct {
	Apps []appDataSourceModel `tfsdk:"apps"`
}

func (d *AppsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apps"
}

func (d *AppsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every aptabase-plus app owned by or shared with the authenticated user - " +
			"useful for discovering apps to `import` into Terraform management.",
		Attributes: map[string]schema.Attribute{
			"apps": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
						"app_key": schema.StringAttribute{
							Computed:  true,
							Sensitive: true,
						},
					},
				},
			},
		},
	}
}

func (d *AppsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*aptabase.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("expected *aptabase.Client, got: %T. Report this issue to the provider maintainers.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *AppsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	apps, err := d.client.ListApps(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Aptabase Apps", err.Error())
		return
	}

	data := appsDataSourceModel{Apps: make([]appDataSourceModel, 0, len(apps))}
	for _, app := range apps {
		data.Apps = append(data.Apps, appDataSourceModel{
			ID:     types.StringValue(app.ID),
			Name:   types.StringValue(app.Name),
			AppKey: types.StringValue(app.AppKey),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
