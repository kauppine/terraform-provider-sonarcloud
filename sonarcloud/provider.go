package sonarcloud

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kauppine/go-sonarcloud/sonarcloud"
)

func New() provider.Provider {
	return &sonarcloudProvider{}
}

type sonarcloudProvider struct {
	configured   bool
	client       *sonarcloud.Client
	organization string
}

type providerData struct {
	Organization types.String `tfsdk:"organization"`
	Token        types.String `tfsdk:"token"`
}

func (p *sonarcloudProvider) Metadata(_ context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "sonarcloud"
}

func (p sonarcloudProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Optional: true,
				Description: "The SonarCloud organization to manage the resources for. This value must be set in the" +
					" `SONARCLOUD_ORGANIZATION` environment variable if left empty.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "The token of a user with admin permissions in the organization. This value must be set in" +
					" the `SONARCLOUD_TOKEN` environment variable if left empty.",
			},
		},
	}
}

func (p *sonarcloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var organization string
	if config.Organization.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create client",
			"Cannot use unknown value as organization",
		)
		return
	}

	if config.Organization.IsNull() {
		organization = os.Getenv("SONARCLOUD_ORGANIZATION")
	} else {
		organization = config.Organization.ValueString()
	}

	var token string
	if config.Token.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create client",
			"Cannot use unknown value as token",
		)
	}

	if config.Token.IsNull() {
		token = os.Getenv("SONARCLOUD_TOKEN")
	} else {
		token = config.Token.ValueString()
	}

	c := sonarcloud.NewClient(organization, token, nil)

	p.client = c
	p.organization = organization
	p.configured = true

	resp.DataSourceData = p
	resp.ResourceData = p
}

func (p *sonarcloudProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserGroupResource,
		NewUserGroupMemberResource,
		NewProjectResource,
		NewProjectLinkResource,
		NewProjectMainBranchResource,
		NewUserTokenResource,
		NewQualityGateResource,
		NewQualityGateSelectionResource,
		NewUserPermissionsResource,
		NewUserGroupPermissionsResource,
		NewWebhookResource,
	}
}

func (p *sonarcloudProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectsDataSource,
		NewProjectLinksDataSource,
		NewUserGroupDataSource,
		NewUserGroupsDataSource,
		NewUserGroupMembersDataSource,
		NewUserGroupPermissionsDataSource,
		NewUserPermissionsDataSource,
		NewQualityGateDataSource,
		NewQualityGatesDataSource,
		NewWebhooksDataSource,
	}
}
