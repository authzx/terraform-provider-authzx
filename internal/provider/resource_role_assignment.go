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

type roleAssignmentResource struct {
	client *client.Client
}

type roleAssignmentModel struct {
	ID        types.String `tfsdk:"id"`
	SubjectID types.String `tfsdk:"subject_id"`
	RoleID    types.String `tfsdk:"role_id"`
}

func NewRoleAssignmentResource() resource.Resource {
	return &roleAssignmentResource{}
}

func (r *roleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_assignment"
}

func (r *roleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a role to a subject. Deleting this resource unassigns the role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: subject_id:role_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subject_id": schema.StringAttribute{
				Required:    true,
				Description: "Subject ID.",
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

func (r *roleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *roleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.AssignRoleToSubject(ctx, plan.SubjectID.ValueString(), plan.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to assign role", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.SubjectID.ValueString() + ":" + plan.RoleID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan roleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.UnassignRoleFromSubject(ctx, state.SubjectID.ValueString(), state.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to unassign role", err.Error())
	}
}

func (r *roleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: subject_id:role_id")
		return
	}

	state := roleAssignmentModel{
		ID:        types.StringValue(req.ID),
		SubjectID: types.StringValue(parts[0]),
		RoleID:    types.StringValue(parts[1]),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
