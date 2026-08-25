package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vengtoo/terraform-provider-vengtoo/internal/client"
)

type groupResource struct {
	client *client.Client
}

type groupModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Metadata    types.String `tfsdk:"metadata"`
}

func NewGroupResource() resource.Resource {
	return &groupResource{}
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Vengtoo group. Groups bundle subjects so roles and policies can be assigned to many subjects at once.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Group name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Group description.",
			},
			"metadata": schema.StringAttribute{
				Optional:    true,
				Description: "Arbitrary metadata, JSON-encoded. Use jsonencode({ team = \"platform\" }).",
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

// metadataRaw validates the JSON at plan time so a malformed value fails here
// rather than as an opaque backend error.
func metadataRaw(v types.String) (json.RawMessage, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	raw := v.ValueString()
	if raw == "" {
		return nil, nil
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("metadata is not valid JSON: %s", raw)
	}
	return json.RawMessage(raw), nil
}

// metadataOrNull maps the backend's default empty metadata to null so a group
// with no metadata does not diff against an omitted attribute.
func metadataOrNull(raw json.RawMessage) types.String {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "{}" || s == "null" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	metadata, err := metadataRaw(plan.Metadata)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse metadata", err.Error())
		return
	}

	group, err := r.client.CreateGroup(ctx, &client.Group{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Metadata:    metadata,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}

	plan.ID = types.StringValue(group.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.GetGroup(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}

	state.Name = types.StringValue(group.Name)
	state.Description = stringOrNull(group.Description)
	state.Metadata = metadataOrNull(group.Metadata)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	metadata, err := metadataRaw(plan.Metadata)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse metadata", err.Error())
		return
	}

	_, err = r.client.UpdateGroup(ctx, state.ID.ValueString(), &client.Group{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Metadata:    metadata,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGroup(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	group, err := r.client.GetGroup(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import group", fmt.Sprintf("Could not find group %s: %s", req.ID, err.Error()))
		return
	}

	state := groupModel{
		ID:          types.StringValue(group.ID),
		Name:        types.StringValue(group.Name),
		Description: stringOrNull(group.Description),
		Metadata:    metadataOrNull(group.Metadata),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
