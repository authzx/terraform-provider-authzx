package provider

import (
	"context"
	"fmt"

	"github.com/authzx/terraform-provider-authzx/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type roleResource struct {
	client *client.Client
}

type roleModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	ApplicationID types.String `tfsdk:"application_id"`
}

func NewRoleResource() resource.Resource {
	return &roleResource{}
}

func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AuthzX role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Role ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Role name (e.g., admin, editor, viewer).",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Role description.",
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "Application this role belongs to.",
			},
		},
	}
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.CreateRole(ctx, &client.Role{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		ApplicationIDs: []string{plan.ApplicationID.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create role", err.Error())
		return
	}

	plan.ID = types.StringValue(role.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.GetRole(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read role", err.Error())
		return
	}

	state.Name = types.StringValue(role.Name)
	state.Description = stringOrNull(role.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateRole(ctx, state.ID.ValueString(), &client.Role{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		ApplicationIDs: []string{plan.ApplicationID.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update role", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRole(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete role", err.Error())
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	role, err := r.client.GetRole(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import role", fmt.Sprintf("Could not find role %s: %s", req.ID, err.Error()))
		return
	}

	state := roleModel{
		ID:            types.StringValue(role.ID),
		Name:          types.StringValue(role.Name),
		Description:   stringOrNull(role.Description),
		ApplicationID: types.StringValue(firstOrEmpty(role.ApplicationIDs)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
