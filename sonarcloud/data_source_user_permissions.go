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

type UserPermissionsDataSource struct {
	p *sonarcloudProvider
}

func NewUserPermissionsDataSource() datasource.DataSource {
	return &UserPermissionsDataSource{}
}

func (*UserPermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_permissions"
}

func (d *UserPermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d UserPermissionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This data source retrieves all the users of an organization and their permissions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The implicit ID of the data source.",
			},
			"project_key": schema.StringAttribute{
				Optional:    true,
				Description: "The key of the project to read the user permissions for.",
			},
			"users": schema.SetNestedAttribute{
				Computed:    true,
				Description: "The users and their permissions.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"login": schema.StringAttribute{
							Computed:    true,
							Description: "The login of the user.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the user.",
						},
						"permissions": schema.SetAttribute{
							ElementType: types.StringType,
							Required:    true,
							Description: "The permissions of the user.",
						},
						"avatar": schema.StringAttribute{
							Computed:    true,
							Description: "The avatar ID of the user.",
						},
					},
				},
			},
		},
	}
}

func (d UserPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DataUserPermissions
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query for permissions
	searchRequest := UserPermissionsSearchRequest{ProjectKey: config.ProjectKey.ValueString()}
	users, err := sonarcloud.GetAll[UserPermissionsSearchRequest, UserPermissionsSearchResponseUser](d.p.client, "/permissions/users", searchRequest, "users")
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get user permissions",
			fmt.Sprintf("The request returned an error: %+v", err),
		)
		return
	}

	result := DataUserPermissions{}
	var allUsers []DataUserPermissionsUser
	for _, user := range users {
		permissionsElems := make([]attr.Value, len(user.Permissions))
		for i, permission := range user.Permissions {
			permissionsElems[i] = types.StringValue(permission)
		}

		allUsers = append(allUsers, DataUserPermissionsUser{
			Login:       types.StringValue(user.Login),
			Name:        types.StringValue(user.Name),
			Permissions: types.SetValueMust(types.StringType, permissionsElems),
			Avatar:      types.StringValue(user.Avatar),
		})
	}
	result.Users = allUsers
	result.ID = types.StringValue(d.p.organization)
	result.ProjectKey = config.ProjectKey

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)

}
