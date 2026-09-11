package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

var _ resource.Resource = &ApiKeyResource{}

func NewApiKeyResource() resource.Resource {
	return &ApiKeyResource{}
}

type ApiKeyResource struct {
	client *aptabase.Client
}

type apiKeyResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	Key       types.String `tfsdk:"key"`
	KeyPrefix types.String `tfsdk:"key_prefix"`
}

func (r *ApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *ApiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An aptabase-plus API key.\n\n" +
			"~> **The plaintext key is only ever returned at creation.** It is stored in Terraform " +
			"state (as any secret-managing resource's state must) but aptabase-plus itself cannot " +
			"return it again. If it's lost outside of state, there is no way to recover it - taint " +
			"and recreate the resource to get a new one. This resource does not support import for " +
			"the same reason: an imported key's plaintext can never be populated.\n\n" +
			"~> Every attribute forces replacement on change - aptabase-plus has no update endpoint " +
			"for keys.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Key ID, assigned by aptabase-plus.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Key name.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 expiry timestamp. Omit for a key that never expires.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The plaintext API key. Only populated on create - see the warning above.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key_prefix": schema.StringAttribute{
				MarkdownDescription: "First 12 characters of the key, for identification without exposing the secret.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ApiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var expiresAt *string
	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() && plan.ExpiresAt.ValueString() != "" {
		v := plan.ExpiresAt.ValueString()
		expiresAt = &v
	}

	created, err := r.client.CreateApiKey(ctx, plan.Name.ValueString(), expiresAt)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Aptabase API Key", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	plan.Key = types.StringValue(created.Key)
	plan.KeyPrefix = types.StringValue(created.KeyPrefix)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := r.client.ListApiKeys(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Aptabase API Keys", err.Error())
		return
	}

	found := false
	for _, k := range keys {
		if k.ID == state.ID.ValueString() {
			state.Name = types.StringValue(k.Name)
			state.KeyPrefix = types.StringValue(k.KeyPrefix)
			found = true
			break
		}
	}
	if !found {
		// aptabase-plus's list endpoints are unpaginated (verified against the
		// server source); if that changes, this inference breaks and could
		// destructively recreate resources still past page one.
		resp.State.RemoveResource(ctx)
		return
	}

	// ExpiresAt is intentionally not reconciled from the list response here -
	// the attribute is RequiresReplace, so undetected drift on it is low-stakes.
	// Key is intentionally not overwritten - the list endpoint never
	// returns it, only the create response does.
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ApiKeyResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update Call",
		"aptabase_api_key has no updatable attributes; every attribute forces replacement. "+
			"This is a provider bug - please report it.")
}

func (r *ApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteApiKey(ctx, state.ID.ValueString()); err != nil {
		var apiErr *aptabase.APIError
		if errors.As(err, &apiErr) && apiErr.NotFound() {
			return
		}
		resp.Diagnostics.AddError("Error Deleting Aptabase API Key", err.Error())
	}
}
