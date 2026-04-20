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

type subjectResource struct {
	client *client.Client
}

type subjectModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	ApplicationID types.String `tfsdk:"application_id"`
	ExternalID    types.String `tfsdk:"external_id"`
}

func NewSubjectResource() resource.Resource {
	return &subjectResource{}
}

func (r *subjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subject"
}

func (r *subjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AuthzX subject (user, service, device).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Subject ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Subject name.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Subject type (e.g., user, service, device).",
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "Application this subject belongs to.",
			},
			"external_id": schema.StringAttribute{
				Optional:    true,
				Description: "Your system's reference ID for this subject. Usable in /v1/authorize as an alternative to the subject's UUID.",
			},
		},
	}
}

func (r *subjectResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func firstOrEmpty(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

// stringOrNull converts a backend string to a types.String, treating "" as null.
// This avoids spurious drift when an Optional field is omitted in the user's
// plan (null) but the backend returns "".
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (r *subjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan subjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := r.client.CreateSubject(ctx, &client.Subject{
		Name:           plan.Name.ValueString(),
		Type:           plan.Type.ValueString(),
		ApplicationIDs: []string{plan.ApplicationID.ValueString()},
		ExternalID:     plan.ExternalID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create subject", err.Error())
		return
	}

	plan.ID = types.StringValue(s.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *subjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state subjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := r.client.GetSubject(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read subject", err.Error())
		return
	}

	state.Name = types.StringValue(s.Name)
	state.Type = types.StringValue(s.Type)
	state.ExternalID = stringOrNull(s.ExternalID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *subjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan subjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state subjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateSubject(ctx, state.ID.ValueString(), &client.Subject{
		Name:           plan.Name.ValueString(),
		Type:           plan.Type.ValueString(),
		ApplicationIDs: []string{plan.ApplicationID.ValueString()},
		ExternalID:     plan.ExternalID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update subject", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *subjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state subjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSubject(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete subject", err.Error())
	}
}

func (r *subjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	s, err := r.client.GetSubject(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import subject", fmt.Sprintf("Could not find subject %s: %s", req.ID, err.Error()))
		return
	}

	state := subjectModel{
		ID:            types.StringValue(s.ID),
		Name:          types.StringValue(s.Name),
		Type:          types.StringValue(s.Type),
		ApplicationID: types.StringValue(firstOrEmpty(s.ApplicationIDs)),
		ExternalID:    stringOrNull(s.ExternalID),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
