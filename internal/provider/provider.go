// Package provider implements the Pagerly Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

var _ provider.Provider = &pagerlyProvider{}

type pagerlyProvider struct {
	version string
}

type providerModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

// New returns the provider constructor providerserver expects.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &pagerlyProvider{version: version}
	}
}

func (p *pagerlyProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pagerly"
	resp.Version = p.version
}

func (p *pagerlyProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Pagerly on-call configuration: teams and rotations, escalation policies, " +
			"alert integrations, services, group on-calls and the user directory. " +
			"Authenticates with an organisation API key created by an org admin under API Keys in the dashboard.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "Pagerly API key. Falls back to the PAGERLY_API_KEY environment variable. " +
					"The key acts as the admin who created it.",
				Optional:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				Description: "API host, for self-hosted or test deployments. Falls back to PAGERLY_BASE_URL, " +
					"then to https://api.pagerly.io.",
				Optional: true,
			},
		},
	}
}

func (p *pagerlyProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("PAGERLY_API_KEY")
	if !cfg.APIKey.IsNull() && !cfg.APIKey.IsUnknown() {
		apiKey = cfg.APIKey.ValueString()
	}
	baseURL := os.Getenv("PAGERLY_BASE_URL")
	if !cfg.BaseURL.IsNull() && !cfg.BaseURL.IsUnknown() {
		baseURL = cfg.BaseURL.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing Pagerly API key",
			"Set api_key in the provider block or the PAGERLY_API_KEY environment variable. "+
				"An org admin creates keys under API Keys in the Pagerly dashboard.")
		return
	}

	c := client.New(baseURL, apiKey, "terraform-provider-pagerly/"+p.version)
	if _, err := c.Me(ctx); err != nil {
		resp.Diagnostics.AddError("Pagerly API key was not accepted", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *pagerlyProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newTeamResource,
		newEscalationPolicyResource,
		newAlertIntegrationResource,
		newServiceResource,
		newOncallGroupResource,
		newUserResource,
	}
}

func (p *pagerlyProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newTeamDataSource,
		newOncallDataSource,
	}
}
