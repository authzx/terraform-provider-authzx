package provider

import (
	"context"
	"strings"

	"github.com/authzx/terraform-provider-authzx/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type policyAssignmentResource struct {
	client *client.Client
}

type policyAssignmentModel struct {
	ID         types.String `tfsdk:"id"`
	PolicyID   types.String `tfsdk:"policy_id"`
	EntityType types.String `tfsdk:"entity_type"`
	EntityID   types.String `tfsdk:"entity_id"`
}

func NewPolicyAssignmentResource() resource.Resource {
	return &policyAssignmentResource{}
}

func (r *policyAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_assignment"
}

func (r *policyAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a policy to a role, subject, or group. Deleting this resource unassigns the policy.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: entity_type:entity_id:policy_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"policy_id": schema.StringAttribute{
				Required:    true,
				Description: "Policy to assign.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"entity_type": schema.StringAttribute{
				Required:    true,
				Description: "Target entity type: role, entity, or group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"entity_id": schema.StringAttribute{
				Required:    true,
				Description: "Target entity ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *policyAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *policyAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.AssignPolicy(ctx, &client.PolicyAssignment{
		PolicyIDs:  []string{plan.PolicyID.ValueString()},
		EntityType: plan.EntityType.ValueString(),
		EntityID:   plan.EntityID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to assign policy", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.EntityType.ValueString() + ":" + plan.EntityID.ValueString() + ":" + plan.PolicyID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// The assignment is implicit — if the policy and entity still exist, the assignment exists.
	// No separate read endpoint for a single assignment.
	var state policyAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *policyAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replace, so this should never be called.
	var plan policyAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.UnassignPolicy(ctx,
		state.EntityType.ValueString(),
		state.EntityID.ValueString(),
		state.PolicyID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to unassign policy", err.Error())
	}
}

func (r *policyAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: entity_type:entity_id:policy_id")
		return
	}

	state := policyAssignmentModel{
		ID:         types.StringValue(req.ID),
		EntityType: types.StringValue(parts[0]),
		EntityID:   types.StringValue(parts[1]),
		PolicyID:   types.StringValue(parts[2]),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
