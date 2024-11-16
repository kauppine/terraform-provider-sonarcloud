package sonarcloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/project_branches"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProjectMainBranchResource struct {
	p *sonarcloudProvider
}

func NewProjectMainBranchResource() resource.Resource {
	return &ProjectMainBranchResource{}
}

func (*ProjectMainBranchResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_main_branch"
}

func (d *ProjectMainBranchResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r ProjectMainBranchResource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: `This resource manages a project main branch.

Note that certain operations, such as the deletion of a project's main branch configuration, may
not be permitted by the SonarCloud web API, or may require admin permissions.
		`,
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"name": {
				Type:        types.StringType,
				Required:    true,
				Description: "The name of the project main branch.",
				Validators: []tfsdk.AttributeValidator{
					stringLengthBetween(1, 255),
				},
			},
			"project_key": {
				Type:        types.StringType,
				Required:    true,
				Description: "The key of the project.",
				Validators: []tfsdk.AttributeValidator{
					stringLengthBetween(1, 400),
				},
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
			},
		},
	}, nil
}

func (r ProjectMainBranchResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan ProjectMainBranch
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := project_branches.RenameRequest{
		Project: plan.ProjectKey.ValueString(),
		Name:    plan.Name.ValueString(),
	}

	err := r.p.client.ProjectBranches.Rename(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the main project branch",
			fmt.Sprintf("The Rename request returned an error: %+v", err),
		)
		return
	}

	var result = ProjectMainBranch{
		ID:         types.StringValue(plan.Name.ValueString()),
		Name:       types.StringValue(plan.Name.ValueString()),
		ProjectKey: types.StringValue(plan.ProjectKey.ValueString()),
	}
	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}

func (r ProjectMainBranchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Retrieve values from state
	var state ProjectMainBranch
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := project_branches.ListRequest{
		Project: state.ProjectKey.ValueString(),
	}

	response, err := r.p.client.ProjectBranches.List(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the project branches",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	// Check if the main branch matches the declared main branch
	if result, ok := findProjectMainBranch(response, state.Name.ValueString(), state.ProjectKey.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r ProjectMainBranchResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from state
	var state ProjectMainBranch
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from plan
	var plan ProjectMainBranch
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := changedAttrs(req, diags)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, ok := changed["name"]; !ok {
		resp.Diagnostics.AddError(
			"Name from plan does not differ from state.",
			"This should not be possible and indicates an issue with the provider. Please contact the developers.")
	}

	request := project_branches.RenameRequest{
		Project: plan.ProjectKey.ValueString(),
		Name:    plan.Name.ValueString(),
	}

	err := r.p.client.ProjectBranches.Rename(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not update the main project branch",
			fmt.Sprintf("The Rename request returned an error: %+v", err),
		)
		return
	}

	// In the absence of an error we assume that the main project branch was updated.
	// The alternative would be to query the API again to verify this.
	// (The rename-response does not have a return value.)
	// As the API seems to be eventually consistent, this results in flaky behaviour, so we just keep it simple for now.
	var result = ProjectMainBranch{
		ID:         types.StringValue(plan.Name.ValueString()),
		Name:       types.StringValue(plan.Name.ValueString()),
		ProjectKey: types.StringValue(plan.ProjectKey.ValueString()),
	}
	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}

func (r ProjectMainBranchResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProjectMainBranch
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// NOTE: according to docs, it's not possible to DELETE main branches, at least not without admin privilege
	// Therefore, we simply remove the main branch from state, per recommendation of @reinoudk.
	resp.State.RemoveResource(ctx)
}

func (r ProjectMainBranchResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: name,project_key. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_key"), idParts[1])...)
}
