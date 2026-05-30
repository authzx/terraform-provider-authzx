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

type namespaceResource struct {
	client *client.Client
}

type namespaceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func NewNamespaceResource() resource.Resource {
	return &namespaceResource{}
}

func (r *namespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (r *namespaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AuthzX namespace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Namespace ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Namespace name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Namespace description.",
			},
		},
	}
}

func (r *namespaceResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func (r *namespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, err := r.client.CreateNamespace(ctx, &client.Namespace{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	plan.ID = types.StringValue(ns.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *namespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, err := r.client.GetNamespace(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read namespace", err.Error())
		return
	}

	state.Name = types.StringValue(ns.Name)
	state.Description = stringOrNull(ns.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *namespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state namespaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	_, err := r.client.UpdateNamespace(ctx, state.ID.ValueString(), &client.Namespace{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *namespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state namespaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteNamespace(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
	}
}

func (r *namespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ns, err := r.client.GetNamespace(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import namespace", fmt.Sprintf("Could not find namespace %s: %s", req.ID, err.Error()))
		return
	}

	state := namespaceModel{
		ID:          types.StringValue(ns.ID),
		Name:        types.StringValue(ns.Name),
		Description: stringOrNull(ns.Description),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
