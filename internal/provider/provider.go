package provider

import (
	"context"
	"os"

	"github.com/authzx/terraform-provider-authzx/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type authzxProvider struct{}

type authzxProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New() provider.Provider {
	return &authzxProvider{}
}

func (p *authzxProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "authzx"
}

func (p *authzxProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage AuthzX authorization resources — applications, subjects, resources, roles, and policies.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "AuthzX API key. Can also be set via AUTHZX_API_KEY env var.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "AuthzX API base URL. Defaults to https://api.authzx.com.",
				Optional:    true,
			},
		},
	}
}

func (p *authzxProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config authzxProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("AUTHZX_API_KEY")
	}
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing API key", "Set api_key in the provider block or AUTHZX_API_KEY environment variable")
		return
	}

	baseURL := "https://api.authzx.com"
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	c := client.New(apiKey, baseURL)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *authzxProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewApplicationResource,
		NewResourceTypeResource,
		NewResourceResource,
		NewSubjectResource,
		NewRoleResource,
		NewGroupResource,
		NewPolicyResource,
		NewPolicyAssignmentResource,
		NewRoleAssignmentResource,
	}
}

func (p *authzxProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
