package sonarcloud

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/user_groups"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserGroupsDataSource struct {
	p *sonarcloudProvider
}

func NewUserGroupsDataSource() datasource.DataSource {
	return &UserGroupsDataSource{}
}

func (*UserGroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_groups"
}

func (d *UserGroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d UserGroupsDataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This data source retrieves a list of user groups for the configured organization.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"groups": {
				Computed:    true,
				Description: "The groups of this organization.",
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
					"members_count": {
						Type:        types.Float64Type,
						Computed:    true,
						Description: "The number of members in this user group.",
					},
					"default": {
						Type:        types.BoolType,
						Computed:    true,
						Description: "Whether new members are added to this user group per default or not.",
					},
				}),
			},
		},
	}, nil
}

func (d UserGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var diags diag.Diagnostics

	request := user_groups.SearchRequest{}

	res, err := d.p.client.UserGroups.SearchAll(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read user_groups",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	result := Groups{}
	allGroups := make([]Group, len(res.Groups))
	for i, group := range res.Groups {
		allGroups[i] = Group{
			ID:           types.StringValue(big.NewFloat(group.Id).String()),
			Default:      types.BoolValue(group.Default),
			Description:  types.StringValue(group.Description),
			MembersCount: types.NumberValue(big.NewFloat(group.MembersCount)),
			Name:         types.StringValue(group.Name),
		}
	}
	result.Groups = allGroups
	result.ID = types.StringValue(d.p.organization)

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
