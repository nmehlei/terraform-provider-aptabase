package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

var (
	_ resource.Resource                = &AppResource{}
	_ resource.ResourceWithImportState = &AppResource{}
)

func NewAppResource() resource.Resource {
	return &AppResource{}
}

type AppResource struct {
	client *aptabase.Client
}

type appResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Icon   types.String `tfsdk:"icon"`
	AppKey types.String `tfsdk:"app_key"`
}

func (r *AppResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *AppResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An aptabase-plus app.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "App ID, assigned by aptabase-plus.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "App name.",
				Required:            true,
			},
			"icon": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded PNG icon. Write-only: aptabase-plus does not return " +
					"the icon's original bytes, only an internal storage path, so this provider cannot detect " +
					"drift on this attribute - changing it in config always re-applies it, but an icon changed " +
					"outside Terraform won't show as a diff.",
				Optional: true,
			},
			"app_key": schema.StringAttribute{
				MarkdownDescription: "SDK ingestion key for this app.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *AppResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*aptabase.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("expected *aptabase.Client, got: %T. Report this issue to the provider maintainers.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateApp(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Aptabase App", err.Error())
		return
	}

	if !plan.Icon.IsNull() && !plan.Icon.IsUnknown() && plan.Icon.ValueString() != "" {
		updated, err := r.client.UpdateApp(ctx, created.ID, created.Name, plan.Icon.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error Setting Aptabase App Icon", err.Error())
			return
		}
		created = updated
	}

	plan.ID = types.StringValue(created.ID)
	plan.Name = types.StringValue(created.Name)
	plan.AppKey = types.StringValue(created.AppKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApp(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *aptabase.APIError
		if errors.As(err, &apiErr) && apiErr.NotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Aptabase App", err.Error())
		return
	}

	state.Name = types.StringValue(app.Name)
	state.AppKey = types.StringValue(app.AppKey)
	// Icon is intentionally not overwritten from the server - see the
	// schema's MarkdownDescription: aptabase-plus doesn't return it.

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan appResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	icon := ""
	if !plan.Icon.IsNull() && !plan.Icon.IsUnknown() {
		icon = plan.Icon.ValueString()
	}

	updated, err := r.client.UpdateApp(ctx, plan.ID.ValueString(), plan.Name.ValueString(), icon)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Aptabase App", err.Error())
		return
	}

	plan.Name = types.StringValue(updated.Name)
	plan.AppKey = types.StringValue(updated.AppKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteApp(ctx, state.ID.ValueString()); err != nil {
		var apiErr *aptabase.APIError
		if errors.As(err, &apiErr) && apiErr.NotFound() {
			return
		}
		resp.Diagnostics.AddError("Error Deleting Aptabase App", err.Error())
	}
}

func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
