package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/user_groups"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

type UserGroupDataSource struct {
	p *sonarcloudProvider
}

func NewUserGroupDataSource() datasource.DataSource {
	return &UserGroupDataSource{}
}

func (*UserGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

func (d *UserGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d UserGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This data source retrieves a single user group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the user group.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the user group.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "The description of the user group.",
			},
			"members_count": schema.NumberAttribute{
				Computed:    true,
				Description: "The number of members in this user group.",
			},
			"default": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether new members are added to this user group per default or not.",
			},
		},
	}
}

func (d UserGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Retrieve values from config
	var config Group
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := user_groups.SearchRequest{
		Q: config.Name.ValueString(),
	}

	response, err := d.p.client.UserGroups.SearchAll(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the user_group",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if result, ok := findGroup(response, config.Name.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}
