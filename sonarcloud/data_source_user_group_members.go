package sonarcloud

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kauppine/go-sonarcloud/sonarcloud/user_groups"
)

type UserGroupMembersDataSource struct {
	p *sonarcloudProvider
}

func NewUserGroupMembersDataSource() datasource.DataSource {
	return &UserGroupMembersDataSource{}
}

func (*UserGroupMembersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group_members"
}

func (d *UserGroupMembersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d UserGroupMembersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This data source retrieves a list of users for the given group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"group": schema.StringAttribute{
				Required:    true,
				Description: "The name of the group.",
			},
			"users": schema.SetNestedAttribute{
				Computed:    true,
				Description: "The users of the group.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"login": schema.StringAttribute{
							Computed:    true,
							Description: "The login of this user",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of this user",
						},
					},
				},
			},
		},
	}
}

func (d UserGroupMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config Users
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An empty search request retrieves all members
	request := user_groups.UsersRequest{
		Name: config.Group.ValueString(),
	}

	res, err := d.p.client.UserGroups.UsersAll(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read user_group_members.",
			fmt.Sprintf("The UsersAll request returned an error: %+v", err),
		)
		return
	}

	result := Users{}
	allUsers := make([]User, len(res.Users))
	for i, user := range res.Users {
		allUsers[i] = User{
			Login: types.StringValue(user.Login),
			Name:  types.StringValue(user.Name),
		}
	}
	result.Users = allUsers
	result.ID = config.Group
	result.Group = config.Group

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
