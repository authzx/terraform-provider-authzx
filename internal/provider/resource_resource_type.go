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

type resourceTypeResource struct {
	client *client.Client
}

type resourceTypeModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	Actions       types.List   `tfsdk:"actions"`
	ApplicationID types.String `tfsdk:"application_id"`
}

func NewResourceTypeResource() resource.Resource {
	return &resourceTypeResource{}
}

func (r *resourceTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_type"
}

func (r *resourceTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AuthzX resource type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Resource type ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Resource type name (e.g., document, api-endpoint).",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Resource type description.",
			},
			"actions": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Available actions (e.g., read, write, delete).",
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "Application this resource type belongs to.",
			},
		},
	}
}

func (r *resourceTypeResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *resourceTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resourceTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var actions []string
	resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)

	defaultActions := make([]client.Action, len(actions))
	for i, a := range actions {
		defaultActions[i] = client.Action{Name: a, Identifier: a}
	}

	rt, err := r.client.CreateResourceType(ctx, &client.ResourceType{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		DefaultActions: defaultActions,
		ApplicationID:  plan.ApplicationID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create resource type", err.Error())
		return
	}

	plan.ID = types.StringValue(rt.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resourceTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rt, err := r.client.GetResourceType(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource type", err.Error())
		return
	}

	state.Name = types.StringValue(rt.Name)
	state.Description = stringOrNull(rt.Description)

	actionNames := make([]string, len(rt.DefaultActions))
	for i, a := range rt.DefaultActions {
		actionNames[i] = a.Name
	}
	actionsList, diags := types.ListValueFrom(ctx, types.StringType, actionNames)
	resp.Diagnostics.Append(diags...)
	state.Actions = actionsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resourceTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state resourceTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var actions []string
	resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)

	defaultActions := make([]client.Action, len(actions))
	for i, a := range actions {
		defaultActions[i] = client.Action{Name: a, Identifier: a}
	}

	_, err := r.client.UpdateResourceType(ctx, state.ID.ValueString(), &client.ResourceType{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		DefaultActions: defaultActions,
		ApplicationID:  plan.ApplicationID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update resource type", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resourceTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteResourceType(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete resource type", err.Error())
	}
}

func (r *resourceTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	rt, err := r.client.GetResourceType(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import resource type", fmt.Sprintf("Could not find resource type %s: %s", req.ID, err.Error()))
		return
	}

	actionNames := make([]string, len(rt.DefaultActions))
	for i, a := range rt.DefaultActions {
		actionNames[i] = a.Name
	}
	actionsList, diags := types.ListValueFrom(ctx, types.StringType, actionNames)
	resp.Diagnostics.Append(diags...)

	state := resourceTypeModel{
		ID:            types.StringValue(rt.ID),
		Name:          types.StringValue(rt.Name),
		Description:   stringOrNull(rt.Description),
		Actions:       actionsList,
		ApplicationID: types.StringValue(rt.ApplicationID),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
