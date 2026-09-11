package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

var _ datasource.DataSource = &AppDataSource{}

func NewAppDataSource() datasource.DataSource {
	return &AppDataSource{}
}

type AppDataSource struct {
	client *aptabase.Client
}

type appDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	AppKey types.String `tfsdk:"app_key"`
}

func (d *AppDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *AppDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single aptabase-plus app by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "App ID.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "App name.",
				Computed:            true,
			},
			"app_key": schema.StringAttribute{
				MarkdownDescription: "SDK ingestion key for this app.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (d *AppDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AppDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data appDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := d.client.GetApp(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Aptabase App", err.Error())
		return
	}

	data.Name = types.StringValue(app.Name)
	data.AppKey = types.StringValue(app.AppKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
