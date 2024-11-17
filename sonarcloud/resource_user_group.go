package sonarcloud

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/user_groups"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserGroupResource struct {
	p *sonarcloudProvider
}

func NewUserGroupResource() resource.Resource {
	return &UserGroupResource{}
}

func (*UserGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

func (d *UserGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r UserGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This resource manages a user group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the user group.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description for the user group.",
			},
			"default": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the group is the default group or not.",
			},
			"members_count": schema.NumberAttribute{
				Computed:    true,
				Description: "The number of members this group has.",
			},
		},
	}
}

func (r UserGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan Group
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := user_groups.CreateRequest{
		Name:         plan.Name.ValueString(),
		Description:  plan.Description.ValueString(),
		Organization: r.p.organization,
	}

	res, err := r.p.client.UserGroups.Create(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the user_group",
			fmt.Sprintf("The Create request returned an error: %+v", err),
		)
		return
	}

	var result = Group{
		Default:      types.BoolValue(res.Group.Default),
		Description:  types.StringValue(res.Group.Description),
		ID:           types.StringValue(big.NewFloat(res.Group.Id).String()),
		MembersCount: types.NumberValue(big.NewFloat(res.Group.MembersCount)),
		Name:         types.StringValue(res.Group.Name),
	}
	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}

func (r UserGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Retrieve values from state
	var state Group
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := user_groups.SearchRequest{
		Q: state.Name.ValueString(),
	}

	response, err := r.p.client.UserGroups.SearchAll(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the user_group",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if result, ok := findGroup(response, state.Name.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r UserGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from state
	var state Group
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from plan
	var plan Group
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := changedAttrs(req, diags)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	// Note: we skip values that have not been changed
	request := user_groups.UpdateRequest{
		Id: state.ID.ValueString(),
	}

	if _, ok := changed["name"]; ok {
		request.Name = plan.Name.ValueString()
	}
	if _, ok := changed["description"]; ok {
		request.Description = plan.Description.ValueString()
	}

	err := r.p.client.UserGroups.Update(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not update the user_group",
			fmt.Sprintf("The Update request returned an error: %+v", err),
		)
		return
	}

	// We don't have a return value, so we have to query it again
	// Fill in api action struct
	searchRequest := user_groups.SearchRequest{}

	response, err := r.p.client.UserGroups.SearchAll(searchRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the user_group",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if result, ok := findGroup(response, plan.Name.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	}
}

func (r UserGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state Group
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := user_groups.DeleteRequest{
		Id: state.ID.ValueString(),
	}

	err := r.p.client.UserGroups.Delete(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not delete the user_group",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r UserGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
