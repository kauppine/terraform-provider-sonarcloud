package sonarcloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/project_links"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProjectLinkResource struct {
	p *sonarcloudProvider
}

func NewProjectLinkResource() resource.Resource {
	return &ProjectLinkResource{}
}

func (*ProjectLinkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_link"
}

func (d *ProjectLinkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r ProjectLinkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This resource represents a project link.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the link.",
			},
			"project_key": schema.StringAttribute{
				Required:    true,
				Description: "The key of the project to add the link to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name the link.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				Required:    true,
				Description: "The url of the link.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r ProjectLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan ProjectLink
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := project_links.CreateRequest{
		Name:       plan.Name.ValueString(),
		ProjectKey: plan.ProjectKey.ValueString(),
		Url:        plan.Url.ValueString(),
	}

	res, err := r.p.client.ProjectLinks.Create(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the project link",
			fmt.Sprintf("The Create request returned an error: %+v", err),
		)
		return
	}

	link := res.Link
	var result = ProjectLink{
		ID:         types.StringValue(link.Id),
		ProjectKey: plan.ProjectKey,
		Name:       types.StringValue(link.Name),
		Url:        types.StringValue(link.Url),
	}
	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}

func (r ProjectLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Retrieve values from state
	var state ProjectLink
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := project_links.SearchRequest{
		ProjectKey: state.ProjectKey.ValueString(),
	}

	response, err := r.p.client.ProjectLinks.Search(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the project link",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if result, ok := findProjectLink(response, state.ID.ValueString(), state.ProjectKey.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r ProjectLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// NOOP, we always need to recreate
}

func (r ProjectLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProjectLink
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := project_links.DeleteRequest{
		Id: state.ID.ValueString(),
	}
	err := r.p.client.ProjectLinks.Delete(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not delete the project link",
			fmt.Sprintf("The Delete request returned an error: %+v", err),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r ProjectLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: id,project_key. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_key"), idParts[1])...)
}

// findProjectLink returns the link with the given id, if it exists in the response
func findProjectLink(response *project_links.SearchResponse, id, project_key string) (ProjectLink, bool) {
	var result ProjectLink
	ok := false
	for _, link := range response.Links {
		if link.Id == id {
			result = ProjectLink{
				ID:         types.StringValue(link.Id),
				ProjectKey: types.StringValue(project_key),
				Name:       types.StringValue(link.Name),
				Url:        types.StringValue(link.Url),
			}
			ok = true
			break
		}
	}
	return result, ok
}
