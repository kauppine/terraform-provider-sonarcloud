package sonarcloud

import (
	"context"
	"fmt"
	"os"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func (p *sonarcloudProvider) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"organization": {
				Type:     types.StringType,
				Optional: true,
				Description: "The SonarCloud organization to manage the resources for. This value must be set in the" +
					" `SONARCLOUD_ORGANIZATION` environment variable if left empty.",
			},
			"token": {
				Type:      types.StringType,
				Optional:  true,
				Sensitive: true,
				Description: "The token of a user with admin permissions in the organization. This value must be set in" +
					" the `SONARCLOUD_TOKEN` environment variable if left empty.",
			},
		},
	}, nil
}

func (p *sonarcloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var organization string
	if config.Organization.Unknown {
		resp.Diagnostics.AddWarning(
			"Unable to create client",
			"Cannot use unknown value as organization",
		)
		return
	}

	if config.Organization.Null {
		organization = os.Getenv("SONARCLOUD_ORGANIZATION")
	} else {
		organization = config.Organization.Value
	}

	var token string
	if config.Token.Unknown {
		resp.Diagnostics.AddWarning(
			"Unable to create client",
			"Cannot use unknown value as token",
		)
	}

	if config.Token.Null {
		token = os.Getenv("SONARCLOUD_TOKEN")
	} else {
		token = config.Token.Value
	}

	c := sonarcloud.NewClient(organization, token, nil)
	p.client = c
	p.organization = organization
	p.configured = true
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

// toProvider can be used to cast a generic provider.Provider reference to this specific provider.
// This is ideally used in DataSourceType.NewDataSource and ResourceType.NewResource calls.
func toProvider(in any) (*sonarcloudProvider, diag.Diagnostics) {
	if in == nil {
		return nil, nil
	}

	var diags diag.Diagnostics

	p, ok := in.(*sonarcloudProvider)

	if !ok {
		diags.AddError(
			"Unexpected Provider Instance Type",
			fmt.Sprintf("While creating the data source or resource, an unexpected provider type (%T) was received. "+
				"This is always a bug in the provider code and should be reported to the provider developers.", in,
			),
		)
		return nil, diags
	}

	return p, diags
}
