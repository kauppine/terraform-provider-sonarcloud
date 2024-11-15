package sonarcloud

import (
	"context"
	"os"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
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

func (p *sonarcloudProvider) GetResources(_ context.Context) (map[string]provider.ResourceType, diag.Diagnostics) {
	return map[string]provider.ResourceType{
		"sonarcloud_user_group":             resourceUserGroupType{},
		"sonarcloud_user_group_member":      resourceUserGroupMemberType{},
		"sonarcloud_project":                resourceProjectType{},
		"sonarcloud_project_link":           resourceProjectLinkType{},
		"sonarcloud_project_main_branch":    resourceProjectMainBranchType{},
		"sonarcloud_user_token":             resourceUserTokenType{},
		"sonarcloud_quality_gate":           resourceQualityGateType{},
		"sonarcloud_quality_gate_selection": resourceQualityGateSelectionType{},
		"sonarcloud_user_permissions":       resourceUserPermissionsType{},
		"sonarcloud_user_group_permissions": resourceUserGroupPermissionsType{},
		"sonarcloud_webhook":                resourceWebhookType{},
	}, nil
}

func (p *sonarcloudProvider) GetDataSources(_ context.Context) (map[string]provider.DataSourceType, diag.Diagnostics) {
	return map[string]provider.DataSourceType{
		"sonarcloud_projects":               dataSourceProjectsType{},
		"sonarcloud_project_links":          dataSourceProjectLinksType{},
		"sonarcloud_user_group":             dataSourceUserGroupType{},
		"sonarcloud_user_groups":            dataSourceUserGroupsType{},
		"sonarcloud_user_group_members":     dataSourceUserGroupMembersType{},
		"sonarcloud_user_group_permissions": dataSourceUserGroupPermissionsType{},
		"sonarcloud_user_permissions":       dataSourceUserPermissionsType{},
		"sonarcloud_quality_gate":           dataSourceQualityGateType{},
		"sonarcloud_quality_gates":          dataSourceQualityGatesType{},
		"sonarcloud_webhooks":               dataSourceWebhooksType{},
	}, nil
}

type providerData struct {
	Organization types.String `tfsdk:"organization"`
	Token        types.String `tfsdk:"token"`
}
