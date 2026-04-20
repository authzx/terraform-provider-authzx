package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/authzx/terraform-provider-authzx/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type policyResource struct {
	client *client.Client
}

type policyModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Effect         types.String `tfsdk:"effect"`
	Resources      types.List   `tfsdk:"resources"`
	Priority       types.Int64  `tfsdk:"priority"`
	ApplicationID  types.String `tfsdk:"application_id"`
	Actions        types.List   `tfsdk:"actions"`
	ApplicationIDs types.List   `tfsdk:"application_ids"`
	Conditions     types.List   `tfsdk:"conditions"`
}

type policyResourceRefModel struct {
	ResourceID types.String `tfsdk:"resource_id"`
	Actions    types.List   `tfsdk:"actions"`
}

// policyConditionModel mirrors client.PolicyCondition. Value is surfaced as a
// JSON-encoded string because the underlying value is polymorphic (number,
// string, bool, array) and a single TF attribute type can't express that
// cleanly without types.Dynamic. Users write `value_json = jsonencode(100)`.
type policyConditionModel struct {
	Type      types.String `tfsdk:"type"`
	Field     types.String `tfsdk:"field"`
	Operator  types.String `tfsdk:"operator"`
	ValueJSON types.String `tfsdk:"value_json"`
}

func NewPolicyResource() resource.Resource {
	return &policyResource{}
}

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AuthzX policy.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Policy ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Policy name.",
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Policy description.",
			},
			"effect": schema.StringAttribute{
				Required:    true,
				Description: "Policy effect: ALLOW or DENY.",
			},
			"actions": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Policy-level actions (e.g., read, write, delete). Used for app-wide policies.",
			},
			"application_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Application IDs this policy protects. All resources in these apps are covered.",
			},
			"resources": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Resources and actions this policy applies to.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource_id": schema.StringAttribute{
							Required:    true,
							Description: "Resource ID.",
						},
						"actions": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
							Description: "Actions allowed/denied on this resource.",
						},
					},
				},
			},
			"priority": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Policy priority (0-100). Higher priority policies are evaluated first.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Required:    true,
				Description: "Application this policy belongs to.",
			},
			"conditions": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Structured ABAC conditions evaluated when this policy matches. All conditions must pass (AND semantics). Applies to both ALLOW and DENY policies.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:    true,
							Description: "Condition type: resource_attribute, subject_attribute, timeOfDay, ipAddress, geolocation, environment.",
						},
						"field": schema.StringAttribute{
							Optional:    true,
							Description: "Attribute key (for resource_attribute and subject_attribute types). The condition reads input.{resource|subject}.attributes.<field>.",
						},
						"operator": schema.StringAttribute{
							Required:    true,
							Description: "Comparison operator. For *_attribute types: eq, neq, lt, gt, lte, gte, in. For other types: see docs.",
						},
						"value_json": schema.StringAttribute{
							Required:    true,
							Description: "Comparison value, JSON-encoded. Use jsonencode(100) for numbers, jsonencode(\"finance\") for strings, jsonencode([\"a\", \"b\"]) for lists — polymorphic to match the Rego evaluator's value slot.",
						},
					},
				},
			},
		},
	}
}

func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

// toClientConditions converts the TF-side conditions list into the client's
// []PolicyCondition. value_json strings are decoded as json.RawMessage so the
// polymorphic value (number, string, bool, array) round-trips to the backend.
func toClientConditions(ctx context.Context, l types.List) ([]client.PolicyCondition, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var items []policyConditionModel
	diags := l.ElementsAs(ctx, &items, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to parse conditions: %s", diags.Errors())
	}
	out := make([]client.PolicyCondition, len(items))
	for i, c := range items {
		raw := c.ValueJSON.ValueString()
		if raw == "" {
			raw = "null"
		}
		// Validate it's well-formed JSON now so we fail at plan rather than a
		// confusing 500 from the backend later.
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("conditions[%d].value_json is not valid JSON: %s", i, raw)
		}
		out[i] = client.PolicyCondition{
			Type:     c.Type.ValueString(),
			Field:    c.Field.ValueString(),
			Operator: c.Operator.ValueString(),
			Value:    json.RawMessage(raw),
		}
	}
	return out, nil
}

// conditionsToList converts the client's []PolicyCondition back into a TF list
// for Read. Preserves the value's JSON shape exactly.
func conditionsToList(ctx context.Context, conds []client.PolicyCondition) (types.List, diag.Diagnostics) {
	attrTypes := map[string]attr.Type{
		"type":       types.StringType,
		"field":      types.StringType,
		"operator":   types.StringType,
		"value_json": types.StringType,
	}
	objType := types.ObjectType{AttrTypes: attrTypes}
	if len(conds) == 0 {
		return types.ListNull(objType), nil
	}
	items := make([]policyConditionModel, len(conds))
	for i, c := range conds {
		field := types.StringNull()
		if c.Field != "" {
			field = types.StringValue(c.Field)
		}
		valueJSON := "null"
		if len(c.Value) > 0 {
			valueJSON = string(c.Value)
		}
		items[i] = policyConditionModel{
			Type:      types.StringValue(c.Type),
			Field:     field,
			Operator:  types.StringValue(c.Operator),
			ValueJSON: types.StringValue(valueJSON),
		}
	}
	return types.ListValueFrom(ctx, objType, items)
}

func toClientResources(ctx context.Context, l types.List) ([]client.PolicyResourceRef, error) {
	var refs []policyResourceRefModel
	diags := l.ElementsAs(ctx, &refs, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to parse resources: %s", diags.Errors())
	}

	result := make([]client.PolicyResourceRef, len(refs))
	for i, ref := range refs {
		var actions []string
		diags := ref.Actions.ElementsAs(ctx, &actions, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to parse actions: %s", diags.Errors())
		}
		result[i] = client.PolicyResourceRef{
			ResourceID: ref.ResourceID.ValueString(),
			Actions:    actions,
		}
	}
	return result, nil
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var resources []client.PolicyResourceRef
	if !plan.Resources.IsNull() && !plan.Resources.IsUnknown() {
		var err error
		resources, err = toClientResources(ctx, plan.Resources)
		if err != nil {
			resp.Diagnostics.AddError("Failed to parse resources", err.Error())
			return
		}
	}

	var actions []string
	if !plan.Actions.IsNull() {
		resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)
	}

	appIDs := []string{plan.ApplicationID.ValueString()}
	if !plan.ApplicationIDs.IsNull() {
		resp.Diagnostics.Append(plan.ApplicationIDs.ElementsAs(ctx, &appIDs, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	conditions, err := toClientConditions(ctx, plan.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse conditions", err.Error())
		return
	}

	policy, err := r.client.CreatePolicy(ctx, &client.Policy{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		Effect:         plan.Effect.ValueString(),
		Resources:      resources,
		Priority:       int(plan.Priority.ValueInt64()),
		Actions:        actions,
		ApplicationIDs: appIDs,
		Conditions:     conditions,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create policy", err.Error())
		return
	}

	plan.ID = types.StringValue(policy.ID)
	plan.Priority = types.Int64Value(int64(policy.Priority))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.GetPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read policy", err.Error())
		return
	}

	state.Name = types.StringValue(policy.Name)
	state.Description = types.StringValue(policy.Description)
	state.Effect = types.StringValue(policy.Effect)
	state.Priority = types.Int64Value(int64(policy.Priority))

	// Convert resources back to list
	resourcesList, diags := resourcesToList(ctx, policy.Resources)
	resp.Diagnostics.Append(diags...)
	state.Resources = resourcesList

	// Actions
	if len(policy.Actions) > 0 {
		actionsList, diags := types.ListValueFrom(ctx, types.StringType, policy.Actions)
		resp.Diagnostics.Append(diags...)
		state.Actions = actionsList
	} else {
		state.Actions = types.ListNull(types.StringType)
	}

	// Application IDs (protected apps)
	if len(policy.ApplicationIDs) > 0 {
		appIDsList, diags := types.ListValueFrom(ctx, types.StringType, policy.ApplicationIDs)
		resp.Diagnostics.Append(diags...)
		state.ApplicationIDs = appIDsList
	} else {
		state.ApplicationIDs = types.ListNull(types.StringType)
	}

	// Conditions — polymorphic value, stored as value_json per element.
	conditionsList, diags := conditionsToList(ctx, policy.Conditions)
	resp.Diagnostics.Append(diags...)
	state.Conditions = conditionsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var resources []client.PolicyResourceRef
	if !plan.Resources.IsNull() && !plan.Resources.IsUnknown() {
		var err error
		resources, err = toClientResources(ctx, plan.Resources)
		if err != nil {
			resp.Diagnostics.AddError("Failed to parse resources", err.Error())
			return
		}
	}

	var actions []string
	if !plan.Actions.IsNull() {
		resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)
	}

	appIDs := []string{plan.ApplicationID.ValueString()}
	if !plan.ApplicationIDs.IsNull() {
		resp.Diagnostics.Append(plan.ApplicationIDs.ElementsAs(ctx, &appIDs, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	conditions, err := toClientConditions(ctx, plan.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse conditions", err.Error())
		return
	}

	_, err = r.client.UpdatePolicy(ctx, state.ID.ValueString(), &client.Policy{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueString(),
		Effect:         plan.Effect.ValueString(),
		Resources:      resources,
		Priority:       int(plan.Priority.ValueInt64()),
		Actions:        actions,
		ApplicationIDs: appIDs,
		Conditions:     conditions,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update policy", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeletePolicy(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete policy", err.Error())
	}
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	policy, err := r.client.GetPolicy(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import policy", fmt.Sprintf("Could not find policy %s: %s", req.ID, err.Error()))
		return
	}

	resourcesList, diags := resourcesToList(ctx, policy.Resources)
	resp.Diagnostics.Append(diags...)

	var actionsList types.List
	if len(policy.Actions) > 0 {
		al, d := types.ListValueFrom(ctx, types.StringType, policy.Actions)
		resp.Diagnostics.Append(d...)
		actionsList = al
	} else {
		actionsList = types.ListNull(types.StringType)
	}

	var appIDsList types.List
	if len(policy.ApplicationIDs) > 0 {
		al, d := types.ListValueFrom(ctx, types.StringType, policy.ApplicationIDs)
		resp.Diagnostics.Append(d...)
		appIDsList = al
	} else {
		appIDsList = types.ListNull(types.StringType)
	}

	state := policyModel{
		ID:             types.StringValue(policy.ID),
		Name:           types.StringValue(policy.Name),
		Description:    types.StringValue(policy.Description),
		Effect:         types.StringValue(policy.Effect),
		Resources:      resourcesList,
		Priority:       types.Int64Value(int64(policy.Priority)),
		ApplicationID:  types.StringValue(firstOrEmpty(policy.ApplicationIDs)),
		Actions:        actionsList,
		ApplicationIDs: appIDsList,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func resourcesToList(ctx context.Context, resources []client.PolicyResourceRef) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"resource_id": types.StringType,
			"actions":     types.ListType{ElemType: types.StringType},
		},
	}

	if len(resources) == 0 {
		return types.ListNull(elemType), nil
	}

	elems := make([]attr.Value, len(resources))
	for i, r := range resources {
		actionsList, _ := types.ListValueFrom(ctx, types.StringType, r.Actions)
		obj, _ := types.ObjectValue(elemType.AttrTypes, map[string]attr.Value{
			"resource_id": types.StringValue(r.ResourceID),
			"actions":     actionsList,
		})
		elems[i] = obj
	}

	list, diags := types.ListValue(elemType, elems)
	return list, diags
}

