package sonarcloud

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/projects"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProjectResource struct {
	p *sonarcloudProvider
}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

func (*ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r ProjectResource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This resource manages a project.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"name": {
				Type:        types.StringType,
				Required:    true,
				Description: "The name of the project. **Warning:** forces project recreation when changed.",
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
				Validators: []tfsdk.AttributeValidator{
					stringLengthBetween(1, 255),
				},
			},
			"key": {
				Type:        types.StringType,
				Required:    true,
				Description: "The key of the project. **Warning**: must be globally unique.",
				Validators: []tfsdk.AttributeValidator{
					stringLengthBetween(1, 400),
				},
			},
			"visibility": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
				Description: "The visibility of the project. Use `private` to only share it with your organization." +
					" Use `public` if the project should be visible to everyone. Defaults to the organization's default visibility." +
					" **Note:** private projects are only available when you have a SonarCloud subscription.",
				Validators: []tfsdk.AttributeValidator{
					allowedOptions("public", "private"),
				},
			},
		},
	}, nil
}

func (r ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan Project
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := projects.CreateRequest{
		Name:         plan.Name.Value,
		Organization: r.p.organization,
		Project:      plan.Key.Value,
		Visibility:   plan.Visibility.Value,
	}

	res, err := r.p.client.Projects.Create(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the project",
			fmt.Sprintf("The Create request returned an error: %+v", err),
		)
		return
	}

	var result = Project{
		ID:         types.String{Value: res.Project.Key},
		Name:       types.String{Value: res.Project.Name},
		Key:        types.String{Value: res.Project.Key},
		Visibility: types.String{Value: plan.Visibility.Value},
	}
	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}

func (r ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Retrieve values from state
	var state Project
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := projects.SearchRequest{
		Projects: state.Key.Value,
	}

	response, err := r.p.client.Projects.SearchAll(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the project",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if result, ok := findProject(response, state.Key.Value); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from state
	var state Project
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from plan
	var plan Project
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := changedAttrs(req, diags)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, ok := changed["key"]; ok {
		request := projects.UpdateKeyRequest{
			From: state.Key.Value,
			To:   plan.Key.Value,
		}

		err := r.p.client.Projects.UpdateKey(request)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not update the project key",
				fmt.Sprintf("The UpdateKey request returned an error: %+v", err),
			)
			return
		}
	}

	if _, ok := changed["visibility"]; ok {
		request := projects.UpdateVisibilityRequest{
			Project:    plan.Key.Value,
			Visibility: plan.Visibility.Value,
		}

		err := r.p.client.Projects.UpdateVisibility(request)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not update the project visibility",
				fmt.Sprintf("The UpdateVisibility request returned an error: %+v", err),
			)
			return
		}
	}

	// We don't have a return value, so we have to query it again
	// Fill in api action struct
	searchRequest := projects.SearchRequest{}

	response, err := r.p.client.Projects.SearchAll(searchRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the project",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if result, ok := findProject(response, plan.Key.Value); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	}
}

func (r ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state Project
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := projects.DeleteRequest{
		Project: state.Key.Value,
	}

	err := r.p.client.Projects.Delete(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not delete the project",
			fmt.Sprintf("The Delete request returned an error: %+v", err),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
