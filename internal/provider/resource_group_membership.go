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

type groupMembershipResource struct {
	client *client.Client
}

type groupMembershipModel struct {
	ID       types.String `tfsdk:"id"`
	GroupID  types.String `tfsdk:"group_id"`
	EntityID types.String `tfsdk:"entity_id"`
}

func NewGroupMembershipResource() resource.Resource {
	return &groupMembershipResource{}
}

func (r *groupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

func (r *groupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adds a subject to a group. Deleting this resource removes the subject from the group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: group_id:entity_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Required:    true,
				Description: "Group to add the subject to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"entity_id": schema.StringAttribute{
				Required:    true,
				Description: "Subject to add.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *groupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *groupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AddEntityToGroup(ctx, plan.GroupID.ValueString(), plan.EntityID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to add subject to group", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.GroupID.ValueString() + ":" + plan.EntityID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := groupHasEntity(ctx, r.client, state.GroupID.ValueString(), state.EntityID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group membership", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replace, so this should never be called.
	var plan groupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveEntityFromGroup(ctx, state.GroupID.ValueString(), state.EntityID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to remove subject from group", err.Error())
	}
}

func (r *groupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: group_id:entity_id")
		return
	}

	found, err := groupHasEntity(ctx, r.client, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Failed to import group membership", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to import group membership", "Subject "+parts[1]+" is not a member of group "+parts[0])
		return
	}

	state := groupMembershipModel{
		ID:       types.StringValue(req.ID),
		GroupID:  types.StringValue(parts[0]),
		EntityID: types.StringValue(parts[1]),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func groupHasEntity(ctx context.Context, c *client.Client, groupID, entityID string) (bool, error) {
	entities, err := c.ListGroupEntities(ctx, groupID)
	if err != nil {
		return false, err
	}
	for _, e := range entities {
		if e.ID == entityID {
			return true, nil
		}
	}
	return false, nil
}
