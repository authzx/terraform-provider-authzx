package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vengtoo/terraform-provider-vengtoo/internal/client"
)

// Data sources let a config reference objects it does not manage — e.g. attach
// a static policy to a subject the application created at runtime. Lookup is by
// ID; there is no lookup-by-external_id endpoint yet.

func dsClient(req datasource.ConfigureRequest) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, _ := req.ProviderData.(*client.Client)
	return c
}

// namedDataModel fits any object exposing id/name/description.
type namedDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func namedDataSchema(kind string) schema.Schema {
	return schema.Schema{
		Description: "Look up an existing Vengtoo " + kind + " by ID.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true, Description: kind + " ID."},
			"name":        schema.StringAttribute{Computed: true, Description: kind + " name."},
			"description": schema.StringAttribute{Computed: true, Description: kind + " description."},
		},
	}
}

// subject

type subjectDataSource struct{ client *client.Client }

func NewSubjectDataSource() datasource.DataSource { return &subjectDataSource{} }

func (d *subjectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subject"
}

func (d *subjectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}

func (d *subjectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up an existing Vengtoo subject by ID — e.g. to attach a static policy to a subject your application created.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true, Description: "Subject ID."},
			"name":        schema.StringAttribute{Computed: true, Description: "Subject name."},
			"type":        schema.StringAttribute{Computed: true, Description: "Subject type."},
			"external_id": schema.StringAttribute{Computed: true, Description: "Customer-supplied external ID, if any."},
		},
	}
}

func (d *subjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data subjectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := d.client.GetSubject(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read subject", err.Error())
		return
	}
	data.Name = types.StringValue(s.Name)
	data.Type = types.StringValue(s.Type)
	data.ExternalID = stringOrNull(s.ExternalID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// resource

type resourceDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	NamespaceID types.String `tfsdk:"namespace_id"`
	ExternalID  types.String `tfsdk:"external_id"`
}

type resourceDataSource struct{ client *client.Client }

func NewResourceDataSource() datasource.DataSource { return &resourceDataSource{} }

func (d *resourceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func (d *resourceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}

func (d *resourceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up an existing Vengtoo resource by ID.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Required: true, Description: "Resource ID."},
			"name":         schema.StringAttribute{Computed: true, Description: "Resource name."},
			"type":         schema.StringAttribute{Computed: true, Description: "Resource type ID."},
			"namespace_id": schema.StringAttribute{Computed: true, Description: "Namespace the resource belongs to."},
			"external_id":  schema.StringAttribute{Computed: true, Description: "Customer-supplied external ID, if any."},
		},
	}
}

func (d *resourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data resourceDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r, err := d.client.GetResource(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource", err.Error())
		return
	}
	data.Name = types.StringValue(r.Name)
	data.Type = types.StringValue(r.Type)
	data.NamespaceID = stringOrNull(r.ApplicationID)
	data.ExternalID = stringOrNull(r.ExternalID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// policy

type policyDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Effect      types.String `tfsdk:"effect"`
}

type policyDataSource struct{ client *client.Client }

func NewPolicyDataSource() datasource.DataSource { return &policyDataSource{} }

func (d *policyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *policyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}

func (d *policyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up an existing Vengtoo policy by ID.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true, Description: "Policy ID."},
			"name":        schema.StringAttribute{Computed: true, Description: "Policy name."},
			"description": schema.StringAttribute{Computed: true, Description: "Policy description."},
			"effect":      schema.StringAttribute{Computed: true, Description: "ALLOW or DENY."},
		},
	}
}

func (d *policyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data policyDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := d.client.GetPolicy(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read policy", err.Error())
		return
	}
	data.Name = types.StringValue(p.Name)
	data.Description = stringOrNull(p.Description)
	data.Effect = types.StringValue(p.Effect)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// role / namespace / group / resource_type (id/name/description)

type roleDataSource struct{ client *client.Client }

func NewRoleDataSource() datasource.DataSource { return &roleDataSource{} }

func (d *roleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}
func (d *roleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}
func (d *roleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = namedDataSchema("role")
}
func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data namedDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r, err := d.client.GetRole(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read role", err.Error())
		return
	}
	data.Name = types.StringValue(r.Name)
	data.Description = stringOrNull(r.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type namespaceDataSource struct{ client *client.Client }

func NewNamespaceDataSource() datasource.DataSource { return &namespaceDataSource{} }

func (d *namespaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}
func (d *namespaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}
func (d *namespaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = namedDataSchema("namespace")
}
func (d *namespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data namedDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := d.client.GetNamespace(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read namespace", err.Error())
		return
	}
	data.Name = types.StringValue(n.Name)
	data.Description = stringOrNull(n.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type groupDataSource struct{ client *client.Client }

func NewGroupDataSource() datasource.DataSource { return &groupDataSource{} }

func (d *groupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}
func (d *groupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}
func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = namedDataSchema("group")
}
func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data namedDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := d.client.GetGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}
	data.Name = types.StringValue(g.Name)
	data.Description = stringOrNull(g.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type resourceTypeDataSource struct{ client *client.Client }

func NewResourceTypeDataSource() datasource.DataSource { return &resourceTypeDataSource{} }

func (d *resourceTypeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_type"
}
func (d *resourceTypeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.client = dsClient(req)
}
func (d *resourceTypeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = namedDataSchema("resource type")
}
func (d *resourceTypeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data namedDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rt, err := d.client.GetResourceType(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource type", err.Error())
		return
	}
	data.Name = types.StringValue(rt.Name)
	data.Description = stringOrNull(rt.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
