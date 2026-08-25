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

type groupRoleAssignmentResource struct {
	client *client.Client
}

type groupRoleAssignmentModel struct {
	ID      types.String `tfsdk:"id"`
	GroupID types.String `tfsdk:"group_id"`
	RoleID  types.String `tfsdk:"role_id"`
}

func NewGroupRoleAssignmentResource() resource.Resource {
	return &groupRoleAssignmentResource{}
}

func (r *groupRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_role_assignment"
}

func (r *groupRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a role to a group, granting it to every member of the group. Deleting this resource unassigns the role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: group_id:role_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Required:    true,
				Description: "Group to assign the role to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				Required:    true,
				Description: "Role to assign.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *groupRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *groupRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AssignRoleToGroup(ctx, plan.GroupID.ValueString(), plan.RoleID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to assign role to group", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.GroupID.ValueString() + ":" + plan.RoleID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := groupHasRole(ctx, r.client, state.GroupID.ValueString(), state.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group role assignment", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replace, so this should never be called.
	var plan groupRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveRoleFromGroup(ctx, state.GroupID.ValueString(), state.RoleID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to unassign role from group", err.Error())
	}
}

func (r *groupRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: group_id:role_id")
		return
	}

	found, err := groupHasRole(ctx, r.client, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Failed to import group role assignment", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to import group role assignment", "Role "+parts[1]+" is not assigned to group "+parts[0])
		return
	}

	state := groupRoleAssignmentModel{
		ID:      types.StringValue(req.ID),
		GroupID: types.StringValue(parts[0]),
		RoleID:  types.StringValue(parts[1]),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func groupHasRole(ctx context.Context, c *client.Client, groupID, roleID string) (bool, error) {
	roles, err := c.ListGroupRoles(ctx, groupID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role.ID == roleID {
			return true, nil
		}
	}
	return false, nil
}
