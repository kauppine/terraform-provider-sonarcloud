package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

func (d UserGroupPermissionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This data source retrieves all the user groups and their permissions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The implicit ID of the data source.",
			},
			"project_key": schema.StringAttribute{
				Optional:    true,
				Description: "The key of the project to read the user group permissions for.",
			},
			"groups": schema.SetNestedAttribute{
				Computed:    true,
				Description: "The groups and their permissions.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the user group.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the user group.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the user group.",
						},
						"permissions": schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "The permissions of this user group.",
						},
					},
				},
			},
		},
	}
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
