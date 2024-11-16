package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/user_groups"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserGroupMembersDataSource struct {
	p sonarcloudProvider
}

func NewUserGroupMembersDataSource() datasource.DataSource {
	return &UserGroupMembersDataSource{}
}

func (*UserGroupMembersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group_members"
}

func (d UserGroupMembersDataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This data source retrieves a list of users for the given group.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"group": {
				Type:        types.StringType,
				Required:    true,
				Description: "The name of the group.",
			},
			"users": {
				Computed:    true,
				Description: "The users of the group.",
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"login": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The login of this user",
					},
					"name": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The name of this user",
					},
				}),
			},
		},
	}, nil
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
		Name: config.Group.Value,
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
			Login: types.String{Value: user.Login},
			Name:  types.String{Value: user.Name},
		}
	}
	result.Users = allUsers
	result.ID = config.Group
	result.Group = config.Group

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
