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

type resourceResource struct {
	client *client.Client
}

type resourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	Type          types.String `tfsdk:"type"`
	ApplicationID types.String `tfsdk:"application_id"`
	ExternalID    types.String `tfsdk:"external_id"`
}

func NewResourceResource() resource.Resource {
	return &resourceResource{}
}

func (r *resourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func (r *resourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AuthzX resource — a specific instance of a resource type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Resource ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Resource name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Resource description.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Resource type ID.",
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "Application this resource belongs to.",
			},
			"external_id": schema.StringAttribute{
				Optional:    true,
				Description: "Your system's reference ID.",
			},
		},
	}
}

func (r *resourceResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *resourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.CreateResource(ctx, &client.Resource{
		Name:          plan.Name.ValueString(),
		Description:   plan.Description.ValueString(),
		Type:          plan.Type.ValueString(),
		ApplicationID: plan.ApplicationID.ValueString(),
		ExternalID:    plan.ExternalID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create resource", err.Error())
		return
	}

	plan.ID = types.StringValue(res.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetResource(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource", err.Error())
		return
	}

	state.Name = types.StringValue(res.Name)
	state.Description = stringOrNull(res.Description)
	state.Type = types.StringValue(res.Type)
	state.ApplicationID = types.StringValue(res.ApplicationID)
	state.ExternalID = stringOrNull(res.ExternalID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state resourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateResource(ctx, state.ID.ValueString(), &client.Resource{
		Name:          plan.Name.ValueString(),
		Description:   plan.Description.ValueString(),
		Type:          plan.Type.ValueString(),
		ApplicationID: plan.ApplicationID.ValueString(),
		ExternalID:    plan.ExternalID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update resource", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteResource(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete resource", err.Error())
	}
}

func (r *resourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	res, err := r.client.GetResource(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import resource", fmt.Sprintf("Could not find resource %s: %s", req.ID, err.Error()))
		return
	}

	state := resourceModel{
		ID:            types.StringValue(res.ID),
		Name:          types.StringValue(res.Name),
		Description:   stringOrNull(res.Description),
		Type:          types.StringValue(res.Type),
		ApplicationID: types.StringValue(res.ApplicationID),
		ExternalID:    stringOrNull(res.ExternalID),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
