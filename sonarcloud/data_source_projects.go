package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/projects"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProjectsDataSource struct {
	p *sonarcloudProvider
}

func NewProjectsDataSource() datasource.DataSource {
	return &ProjectsDataSource{}
}

func (*ProjectsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *ProjectsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d ProjectsDataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This data source retrieves a list of projects for the configured organization.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"projects": {
				Computed:    true,
				Description: "The projects of this organization.",
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"id": {
						Type:        types.StringType,
						Computed:    true,
						Description: "ID of the project. Equals to the project name.",
					},
					"name": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The name of the project.",
					},
					"key": {
						Type:        types.StringType,
						Required:    true,
						Description: "The key of the project.",
					},
					"visibility": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The visibility of the project.",
					},
				}),
			},
		},
	}, nil
}

func (d ProjectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var diags diag.Diagnostics

	request := projects.SearchRequest{}

	response, err := d.p.client.Projects.SearchAll(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the project",
			fmt.Sprintf("The SearchAll request returned an error: %+v", err),
		)
		return
	}

	result := Projects{}
	allProjects := make([]Project, len(response.Components))
	for i, component := range response.Components {
		allProjects[i] = Project{
			ID:         types.String{Value: component.Name},
			Name:       types.String{Value: component.Name},
			Key:        types.String{Value: component.Key},
			Visibility: types.String{Value: component.Visibility},
		}
	}
	result.Projects = allProjects
	result.ID = types.String{Value: d.p.organization}

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
