package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserGroupPermissionsDataSource struct {
	p *sonarcloudProvider
}

func NewUserGroupPermissionsDataSource() datasource.DataSource {
	return &UserGroupPermissionsDataSource{}
}

func (*UserGroupPermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group_permissions"
}

func (d *UserGroupPermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*sonarcloudProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *sonarcloud.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
	d.p = provider
}

func (d UserGroupPermissionsDataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This data source retrieves all the user groups and their permissions.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:        types.StringType,
				Computed:    true,
				Description: "The implicit ID of the data source.",
			},
			"project_key": {
				Type:        types.StringType,
				Optional:    true,
				Description: "The key of the project to read the user group permissions for.",
			},
			"groups": {
				Computed:    true,
				Description: "The groups and their permissions.",
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"id": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The ID of the user group.",
					},
					"name": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The name of the user group.",
					},
					"description": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The description of the user group.",
					},
					"permissions": {
						Type:        types.SetType{ElemType: types.StringType},
						Computed:    true,
						Description: "The permissions of this user group.",
					},
				}),
			},
		},
	}, nil
}

func (d UserGroupPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DataUserGroupPermissions
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query for permissions
	searchRequest := UserGroupPermissionsSearchRequest{ProjectKey: config.ProjectKey.ValueString()}
	groups, err := sonarcloud.GetAll[UserGroupPermissionsSearchRequest, UserGroupPermissionsSearchResponseGroup](d.p.client, "/permissions/groups", searchRequest, "groups")
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get user group permissions",
			fmt.Sprintf("The request returned an error: %+v", err),
		)
		return
	}

	result := DataUserGroupPermissions{}
	var allGroups []DataUserGroupPermissionsGroup
	for _, group := range groups {
		permissionsElems := make([]attr.Value, len(group.Permissions))
		for i, permission := range group.Permissions {
			permissionsElems[i] = types.StringValue(permission)
		}

		allGroups = append(allGroups, DataUserGroupPermissionsGroup{
			ID:          types.StringValue(group.Id),
			Name:        types.StringValue(group.Name),
			Description: types.StringValue(group.Description),
			Permissions: types.SetValueMust(types.StringType, permissionsElems),
		})
	}
	result.Groups = allGroups
	result.ID = types.StringValue(d.p.organization)
	result.ProjectKey = config.ProjectKey

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)

}
