package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vengtoo/terraform-provider-vengtoo/internal/client"
)

type roleParentResource struct {
	client *client.Client
}

type roleParentModel struct {
	ID       types.String `tfsdk:"id"`
	RoleID   types.String `tfsdk:"role_id"`
	ParentID types.String `tfsdk:"parent_id"`
}

func NewRoleParentResource() resource.Resource {
	return &roleParentResource{}
}

func (r *roleParentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_parent"
}

func (r *roleParentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Makes one role inherit from another, so the child role picks up the parent's policies. The hierarchy is a DAG — an edge that would create a cycle is rejected.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: role_id:parent_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role_id": schema.StringAttribute{
				Required:    true,
				Description: "Child role, which inherits from the parent.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent role, whose policies are inherited.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *roleParentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *roleParentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleParentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AddRoleParent(ctx, plan.RoleID.ValueString(), plan.ParentID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to add role parent", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.RoleID.ValueString() + ":" + plan.ParentID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleParentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleParentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := roleHasParent(ctx, r.client, state.RoleID.ValueString(), state.ParentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read role parent", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleParentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replace, so this should never be called.
	var plan roleParentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleParentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleParentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveRoleParent(ctx, state.RoleID.ValueString(), state.ParentID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to remove role parent", err.Error())
	}
}

func (r *roleParentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: role_id:parent_id")
		return
	}

	found, err := roleHasParent(ctx, r.client, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Failed to import role parent", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to import role parent", "Role "+parts[0]+" does not inherit from role "+parts[1])
		return
	}

	state := roleParentModel{
		ID:       types.StringValue(req.ID),
		RoleID:   types.StringValue(parts[0]),
		ParentID: types.StringValue(parts[1]),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func roleHasParent(ctx context.Context, c *client.Client, roleID, parentID string) (bool, error) {
	edges, err := c.ListRoleParents(ctx, roleID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		if e.ParentRoleID == parentID {
			return true, nil
		}
	}
	return false, nil
}
