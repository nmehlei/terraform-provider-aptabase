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

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

var (
	_ resource.Resource                = &AppShareResource{}
	_ resource.ResourceWithImportState = &AppShareResource{}
)

func NewAppShareResource() resource.Resource {
	return &AppShareResource{}
}

type AppShareResource struct {
	client *aptabase.Client
}

type appShareResourceModel struct {
	ID    types.String `tfsdk:"id"`
	AppID types.String `tfsdk:"app_id"`
	Email types.String `tfsdk:"email"`
}

func (r *AppShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_share"
}

func (r *AppShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shares an aptabase-plus app with another user by email. " +
			"There is no update - changing either `app_id` or `email` replaces the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`{app_id}/{email}`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "ID of the shared app.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address to share the app with.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *AppShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AppShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := plan.AppID.ValueString()
	email := plan.Email.ValueString()
	if err := r.client.AddShare(ctx, appID, email); err != nil {
		resp.Diagnostics.AddError("Error Creating Aptabase App Share", err.Error())
		return
	}

	plan.ID = types.StringValue(appID + "/" + email)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *AppShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	shares, err := r.client.ListShares(ctx, state.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Aptabase App Shares", err.Error())
		return
	}

	found := false
	for _, s := range shares {
		if strings.EqualFold(s.Email, state.Email.ValueString()) {
			found = true
			break
		}
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *AppShareResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update Call",
		"aptabase_app_share has no updatable attributes; app_id and email both force replacement. "+
			"This is a provider bug - please report it.")
}

func (r *AppShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveShare(ctx, state.AppID.ValueString(), state.Email.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error Deleting Aptabase App Share", err.Error())
	}
}

func (r *AppShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Unexpected Import Identifier",
			fmt.Sprintf("expected import ID in the form <app_id>/<email>, got: %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
