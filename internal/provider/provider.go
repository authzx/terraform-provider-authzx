package provider

import (
	"context"
	"errors"
	"os"

	"github.com/vengtoo/terraform-provider-vengtoo/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultEndpoint = "https://api.vengtoo.com"

type vengtooProvider struct{}

type vengtooProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Endpoint     types.String `tfsdk:"endpoint"`
}

func New() provider.Provider {
	return &vengtooProvider{}
}

func (p *vengtooProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "vengtoo"
}

func (p *vengtooProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Vengtoo authorization resources — namespaces, subjects, resources, roles, and policies. " +
			"Authenticates via OAuth2 Client Credentials (RFC 6749 §4.4).",
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				Description: "Vengtoo OAuth2 client ID. Can also be set via the VENGTOO_CLIENT_ID environment variable.",
				Optional:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "Vengtoo OAuth2 client secret. Can also be set via the VENGTOO_CLIENT_SECRET environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"endpoint": schema.StringAttribute{
				Description: "Vengtoo API endpoint. Defaults to https://api.vengtoo.com. Useful for targeting dev/staging. " +
					"Can also be set via the VENGTOO_ENDPOINT environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *vengtooProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config vengtooProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientID := config.ClientID.ValueString()
	if clientID == "" {
		clientID = os.Getenv("VENGTOO_CLIENT_ID")
	}
	clientSecret := config.ClientSecret.ValueString()
	if clientSecret == "" {
		clientSecret = os.Getenv("VENGTOO_CLIENT_SECRET")
	}

	if clientID == "" {
		resp.Diagnostics.AddError(
			"Missing Vengtoo client_id",
			"Set client_id in the provider block or the VENGTOO_CLIENT_ID environment variable.",
		)
	}
	if clientSecret == "" {
		resp.Diagnostics.AddError(
			"Missing Vengtoo client_secret",
			"Set client_secret in the provider block or the VENGTOO_CLIENT_SECRET environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := config.Endpoint.ValueString()
	if endpoint == "" {
		endpoint = os.Getenv("VENGTOO_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	c := client.New(clientID, clientSecret, endpoint)

	// Exchange credentials up-front so misconfiguration fails at `terraform plan`
	// rather than on the first resource CRUD.
	if err := c.Authenticate(ctx); err != nil {
		if errors.Is(err, client.ErrAuthentication) {
			resp.Diagnostics.AddError(
				"Authentication failed",
				"Authentication failed: check client_id/client_secret. The token endpoint returned invalid_client.",
			)
			return
		}
		resp.Diagnostics.AddError(
			"Failed to obtain access token",
			"Could not exchange client credentials for an access token: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *vengtooProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
		NewResourceTypeResource,
		NewResourceResource,
		NewSubjectResource,
		NewRoleResource,
		NewPolicyResource,
		NewPolicyAssignmentResource,
		NewRoleAssignmentResource,
	}
}

func (p *vengtooProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
