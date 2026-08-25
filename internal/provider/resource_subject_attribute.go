package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vengtoo/terraform-provider-vengtoo/internal/client"
)

type subjectAttributeResource struct {
	client *client.Client
}

type subjectAttributeModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	Description  types.String `tfsdk:"description"`
	EnumOptions  types.List   `tfsdk:"enum_options"`
	Required     types.Bool   `tfsdk:"required"`
	SubjectTypes types.List   `tfsdk:"subject_types"`
}

func NewSubjectAttributeResource() resource.Resource {
	return &subjectAttributeResource{}
}

func (r *subjectAttributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subject_attribute"
}

func (r *subjectAttributeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Vengtoo subject attribute definition — the schema for an ABAC attribute that policy conditions can read off a subject.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Subject attribute ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Attribute key, as referenced by a policy condition's field (e.g., department). Required — the update API treats an empty name as \"unchanged\", so it cannot be cleared.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Attribute value type: string, int, bool, enum, object, or json. Required — the update API treats an empty type as \"unchanged\", so it cannot be cleared.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Attribute description.",
			},
			"enum_options": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Permitted values, for type = enum.",
			},
			"required": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether subjects must carry this attribute.",
			},
			"subject_types": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Subject types this attribute applies to: user, service, device, custom, or ai_agent. Omit to apply to all.",
			},
		},
	}
}

func (r *subjectAttributeResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func stringListOrNull(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}

func (r *subjectAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan subjectAttributeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enumOptions := []string{}
	if !plan.EnumOptions.IsNull() {
		resp.Diagnostics.Append(plan.EnumOptions.ElementsAs(ctx, &enumOptions, false)...)
	}
	subjectTypes := []string{}
	if !plan.SubjectTypes.IsNull() {
		resp.Diagnostics.Append(plan.SubjectTypes.ElementsAs(ctx, &subjectTypes, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	attr, err := r.client.CreateSubjectAttribute(ctx, &client.SubjectAttributeDefinition{
		Name:         plan.Name.ValueString(),
		Type:         plan.Type.ValueString(),
		Description:  plan.Description.ValueString(),
		EnumOptions:  enumOptions,
		Required:     plan.Required.ValueBool(),
		SubjectTypes: subjectTypes,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create subject attribute", err.Error())
		return
	}

	plan.ID = types.StringValue(attr.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *subjectAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state subjectAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attr, err := r.client.GetSubjectAttribute(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read subject attribute", err.Error())
		return
	}

	enumOptions, diags := stringListOrNull(ctx, attr.EnumOptions)
	resp.Diagnostics.Append(diags...)
	subjectTypes, diags := stringListOrNull(ctx, attr.SubjectTypes)
	resp.Diagnostics.Append(diags...)

	state.Name = types.StringValue(attr.Name)
	state.Type = types.StringValue(attr.Type)
	state.Description = stringOrNull(attr.Description)
	state.EnumOptions = enumOptions
	state.Required = types.BoolValue(attr.Required)
	state.SubjectTypes = subjectTypes
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *subjectAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan subjectAttributeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state subjectAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enumOptions := []string{}
	if !plan.EnumOptions.IsNull() {
		resp.Diagnostics.Append(plan.EnumOptions.ElementsAs(ctx, &enumOptions, false)...)
	}
	subjectTypes := []string{}
	if !plan.SubjectTypes.IsNull() {
		resp.Diagnostics.Append(plan.SubjectTypes.ElementsAs(ctx, &subjectTypes, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateSubjectAttribute(ctx, state.ID.ValueString(), &client.SubjectAttributeDefinition{
		Name:         plan.Name.ValueString(),
		Type:         plan.Type.ValueString(),
		Description:  plan.Description.ValueString(),
		EnumOptions:  enumOptions,
		Required:     plan.Required.ValueBool(),
		SubjectTypes: subjectTypes,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update subject attribute", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *subjectAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state subjectAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSubjectAttribute(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete subject attribute", err.Error())
	}
}

func (r *subjectAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	attr, err := r.client.GetSubjectAttribute(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import subject attribute", fmt.Sprintf("Could not find subject attribute %s: %s", req.ID, err.Error()))
		return
	}

	enumOptions, diags := stringListOrNull(ctx, attr.EnumOptions)
	resp.Diagnostics.Append(diags...)
	subjectTypes, diags := stringListOrNull(ctx, attr.SubjectTypes)
	resp.Diagnostics.Append(diags...)

	state := subjectAttributeModel{
		ID:           types.StringValue(attr.ID),
		Name:         types.StringValue(attr.Name),
		Type:         types.StringValue(attr.Type),
		Description:  stringOrNull(attr.Description),
		EnumOptions:  enumOptions,
		Required:     types.BoolValue(attr.Required),
		SubjectTypes: subjectTypes,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
