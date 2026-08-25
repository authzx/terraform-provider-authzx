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

type groupParentResource struct {
	client *client.Client
}

type groupParentModel struct {
	ID       types.String `tfsdk:"id"`
	GroupID  types.String `tfsdk:"group_id"`
	ParentID types.String `tfsdk:"parent_id"`
}

func NewGroupParentResource() resource.Resource {
	return &groupParentResource{}
}

func (r *groupParentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_parent"
}

func (r *groupParentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Makes one group inherit from another, so the child group picks up the parent's roles and policies. The hierarchy is a DAG — an edge that would create a cycle is rejected.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: group_id:parent_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Required:    true,
				Description: "Child group, which inherits from the parent.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent_id": schema.StringAttribute{
				Required:    true,
				Description: "Parent group, whose roles and policies are inherited.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *groupParentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *groupParentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupParentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AddGroupParent(ctx, plan.GroupID.ValueString(), plan.ParentID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to add group parent", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.GroupID.ValueString() + ":" + plan.ParentID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupParentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupParentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := groupHasParent(ctx, r.client, state.GroupID.ValueString(), state.ParentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group parent", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupParentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replace, so this should never be called.
	var plan groupParentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupParentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupParentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveGroupParent(ctx, state.GroupID.ValueString(), state.ParentID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to remove group parent", err.Error())
	}
}

func (r *groupParentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: group_id:parent_id")
		return
	}

	found, err := groupHasParent(ctx, r.client, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Failed to import group parent", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to import group parent", "Group "+parts[0]+" does not inherit from group "+parts[1])
		return
	}

	state := groupParentModel{
		ID:       types.StringValue(req.ID),
		GroupID:  types.StringValue(parts[0]),
		ParentID: types.StringValue(parts[1]),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func groupHasParent(ctx context.Context, c *client.Client, groupID, parentID string) (bool, error) {
	edges, err := c.ListGroupParents(ctx, groupID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		if e.ParentGroupID == parentID {
			return true, nil
		}
	}
	return false, nil
}
