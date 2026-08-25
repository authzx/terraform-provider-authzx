package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/vengtoo/terraform-provider-vengtoo/internal/client"
)

type policyResource struct {
	client *client.Client
}

type policyModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	Effect        types.String `tfsdk:"effect"`
	Resources     types.List   `tfsdk:"resources"`
	ResourceTypes types.List   `tfsdk:"resource_types"`
	Priority      types.Int64  `tfsdk:"priority"`
	Actions       types.List   `tfsdk:"actions"`
	Conditions    types.Object `tfsdk:"conditions"`
}

type policyResourceRefModel struct {
	ResourceID types.String `tfsdk:"resource_id"`
	Actions    types.List   `tfsdk:"actions"`
}

type policyResourceTypeRefModel struct {
	ResourceTypeID types.String `tfsdk:"resource_type_id"`
	Actions        types.List   `tfsdk:"actions"`
}

// attrCheckModel is one {key, op, value} check. value_json is JSON-encoded
// because the value is polymorphic; users write `value_json = jsonencode(100)`.
type attrCheckModel struct {
	Key       types.String `tfsdk:"key"`
	Op        types.String `tfsdk:"op"`
	ValueJSON types.String `tfsdk:"value_json"`
}

type conditionsModel struct {
	SubjectAttrs  types.List `tfsdk:"subject_attrs"`
	ResourceAttrs types.List `tfsdk:"resource_attrs"`
	ContextAttrs  types.List `tfsdk:"context_attrs"`
}

var attrCheckAttrTypes = map[string]attr.Type{
	"key":        types.StringType,
	"op":         types.StringType,
	"value_json": types.StringType,
}

var attrCheckListType = types.ListType{ElemType: types.ObjectType{AttrTypes: attrCheckAttrTypes}}

var conditionsAttrTypes = map[string]attr.Type{
	"subject_attrs":  attrCheckListType,
	"resource_attrs": attrCheckListType,
	"context_attrs":  attrCheckListType,
}

func NewPolicyResource() resource.Resource {
	return &policyResource{}
}

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Vengtoo policy.",
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
			"resource_types": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Resource types and actions this policy applies to (type-level targeting — covers all resources of the type).",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource_type_id": schema.StringAttribute{
							Required:    true,
							Description: "Resource type ID.",
						},
						"actions": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
							Description: "Actions allowed/denied on this resource type.",
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
			"conditions": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Structured ABAC conditions (AND semantics), matched on subject, resource, and request-context attributes. Advanced guards (time windows, MFA, trust level, geo, rate limits, human approval, expression trees) are managed via the API, not Terraform.",
				Attributes: map[string]schema.Attribute{
					"subject_attrs":  attrChecksSchema("subject"),
					"resource_attrs": attrChecksSchema("resource"),
					"context_attrs":  attrChecksSchema("request context"),
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

// attrChecksSchema is the nested schema for one {key, op, value_json} array.
func attrChecksSchema(scope string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional:    true,
		Description: "Attribute checks against the " + scope + " (all must pass).",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"key": schema.StringAttribute{
					Required:    true,
					Description: "Attribute key.",
				},
				"op": schema.StringAttribute{
					Required:    true,
					Description: "Operator: eq, ne, gt, gte, lt, lte, in, not_in, matches.",
					Validators: []validator.String{
						stringvalidator.OneOf("eq", "ne", "gt", "gte", "lt", "lte", "in", "not_in", "matches"),
					},
				},
				"value_json": schema.StringAttribute{
					Required:    true,
					Description: "Comparison value, JSON-encoded: jsonencode(100), jsonencode(\"finance\"), jsonencode([\"a\", \"b\"]).",
				},
			},
		},
	}
}

// toClientConditions converts the TF conditions object into the API's typed
// PolicyConditions. Returns nil when no attribute checks are set.
func toClientConditions(ctx context.Context, o types.Object) (*client.PolicyConditions, error) {
	if o.IsNull() || o.IsUnknown() {
		return nil, nil
	}
	var m conditionsModel
	if diags := o.As(ctx, &m, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, fmt.Errorf("failed to parse conditions: %s", diags.Errors())
	}
	subj, err := toAttrChecks(ctx, m.SubjectAttrs, "subject_attrs")
	if err != nil {
		return nil, err
	}
	res, err := toAttrChecks(ctx, m.ResourceAttrs, "resource_attrs")
	if err != nil {
		return nil, err
	}
	cx, err := toAttrChecks(ctx, m.ContextAttrs, "context_attrs")
	if err != nil {
		return nil, err
	}
	if len(subj) == 0 && len(res) == 0 && len(cx) == 0 {
		return nil, nil
	}
	return &client.PolicyConditions{SubjectAttrs: subj, ResourceAttrs: res, ContextAttrs: cx}, nil
}

func toAttrChecks(ctx context.Context, l types.List, name string) ([]client.AttrCheck, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var items []attrCheckModel
	if diags := l.ElementsAs(ctx, &items, false); diags.HasError() {
		return nil, fmt.Errorf("failed to parse %s: %s", name, diags.Errors())
	}
	out := make([]client.AttrCheck, len(items))
	for i, c := range items {
		raw := c.ValueJSON.ValueString()
		if raw == "" {
			raw = "null"
		}
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("%s[%d].value_json is not valid JSON: %s", name, i, raw)
		}
		out[i] = client.AttrCheck{Key: c.Key.ValueString(), Op: c.Op.ValueString(), Value: json.RawMessage(raw)}
	}
	return out, nil
}

// conditionsToObject converts the API's PolicyConditions back into the TF
// object for Read, preserving each value's JSON shape.
func conditionsToObject(ctx context.Context, c *client.PolicyConditions) (types.Object, diag.Diagnostics) {
	if c == nil || (len(c.SubjectAttrs) == 0 && len(c.ResourceAttrs) == 0 && len(c.ContextAttrs) == 0) {
		return types.ObjectNull(conditionsAttrTypes), nil
	}
	var diags diag.Diagnostics
	subj, d := attrChecksToList(ctx, c.SubjectAttrs)
	diags.Append(d...)
	res, d := attrChecksToList(ctx, c.ResourceAttrs)
	diags.Append(d...)
	cx, d := attrChecksToList(ctx, c.ContextAttrs)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(conditionsAttrTypes), diags
	}
	obj, d := types.ObjectValue(conditionsAttrTypes, map[string]attr.Value{
		"subject_attrs":  subj,
		"resource_attrs": res,
		"context_attrs":  cx,
	})
	diags.Append(d...)
	return obj, diags
}

func attrChecksToList(ctx context.Context, checks []client.AttrCheck) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: attrCheckAttrTypes}
	if len(checks) == 0 {
		return types.ListNull(objType), nil
	}
	items := make([]attrCheckModel, len(checks))
	for i, c := range checks {
		vj := "null"
		if len(c.Value) > 0 {
			vj = string(c.Value)
		}
		items[i] = attrCheckModel{
			Key:       types.StringValue(c.Key),
			Op:        types.StringValue(c.Op),
			ValueJSON: types.StringValue(vj),
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

func toClientResourceTypes(ctx context.Context, l types.List) ([]client.PolicyResourceTypeRef, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var refs []policyResourceTypeRefModel
	diags := l.ElementsAs(ctx, &refs, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to parse resource_types: %s", diags.Errors())
	}
	result := make([]client.PolicyResourceTypeRef, len(refs))
	for i, ref := range refs {
		var actions []string
		diags := ref.Actions.ElementsAs(ctx, &actions, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to parse resource_types[%d].actions: %s", i, diags.Errors())
		}
		result[i] = client.PolicyResourceTypeRef{
			ResourceTypeID: ref.ResourceTypeID.ValueString(),
			Actions:        actions,
		}
	}
	return result, nil
}

func resourceTypesToList(ctx context.Context, rts []client.PolicyResourceTypeRef) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"resource_type_id": types.StringType,
			"actions":          types.ListType{ElemType: types.StringType},
		},
	}
	if len(rts) == 0 {
		return types.ListNull(elemType), nil
	}
	elems := make([]attr.Value, len(rts))
	for i, rt := range rts {
		actionsList, _ := types.ListValueFrom(ctx, types.StringType, rt.Actions)
		obj, _ := types.ObjectValue(elemType.AttrTypes, map[string]attr.Value{
			"resource_type_id": types.StringValue(rt.ResourceTypeID),
			"actions":          actionsList,
		})
		elems[i] = obj
	}
	return types.ListValue(elemType, elems)
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

	resourceTypes, err := toClientResourceTypes(ctx, plan.ResourceTypes)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse resource_types", err.Error())
		return
	}

	var actions []string
	if !plan.Actions.IsNull() {
		resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)
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
		Name:          plan.Name.ValueString(),
		Description:   plan.Description.ValueString(),
		Effect:        plan.Effect.ValueString(),
		Resources:     resources,
		ResourceTypes: resourceTypes,
		Priority:      int(plan.Priority.ValueInt64()),
		Actions:       actions,
		Conditions:    conditions,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create policy", err.Error())
		return
	}

	plan.ID = types.StringValue(policy.ID)
	plan.Priority = types.Int64Value(int64(policy.Priority))

	rtList, diags := resourceTypesToList(ctx, policy.ResourceTypes)
	resp.Diagnostics.Append(diags...)
	plan.ResourceTypes = rtList

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

	// Resource type targets
	rtList, diags := resourceTypesToList(ctx, policy.ResourceTypes)
	resp.Diagnostics.Append(diags...)
	state.ResourceTypes = rtList

	// Actions
	if len(policy.Actions) > 0 {
		actionsList, diags := types.ListValueFrom(ctx, types.StringType, policy.Actions)
		resp.Diagnostics.Append(diags...)
		state.Actions = actionsList
	} else {
		state.Actions = types.ListNull(types.StringType)
	}

	// Conditions — polymorphic value, stored as value_json per element.
	conditionsList, diags := conditionsToObject(ctx, policy.Conditions)
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

	resourceTypes, err := toClientResourceTypes(ctx, plan.ResourceTypes)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse resource_types", err.Error())
		return
	}

	var actions []string
	if !plan.Actions.IsNull() {
		resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)
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
		Name:          plan.Name.ValueString(),
		Description:   plan.Description.ValueString(),
		Effect:        plan.Effect.ValueString(),
		Resources:     resources,
		ResourceTypes: resourceTypes,
		Priority:      int(plan.Priority.ValueInt64()),
		Actions:       actions,
		Conditions:    conditions,
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

	rtList, diags := resourceTypesToList(ctx, policy.ResourceTypes)
	resp.Diagnostics.Append(diags...)

	conditionsList, diags := conditionsToObject(ctx, policy.Conditions)
	resp.Diagnostics.Append(diags...)

	var actionsList types.List
	if len(policy.Actions) > 0 {
		al, d := types.ListValueFrom(ctx, types.StringType, policy.Actions)
		resp.Diagnostics.Append(d...)
		actionsList = al
	} else {
		actionsList = types.ListNull(types.StringType)
	}

	state := policyModel{
		ID:            types.StringValue(policy.ID),
		Name:          types.StringValue(policy.Name),
		Description:   types.StringValue(policy.Description),
		Effect:        types.StringValue(policy.Effect),
		Resources:     resourcesList,
		ResourceTypes: rtList,
		Priority:      types.Int64Value(int64(policy.Priority)),
		Actions:       actionsList,
		Conditions:    conditionsList,
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
