package sonarcloud

import (
	"context"
	"fmt"

	pl "github.com/ArgonGlow/go-sonarcloud/sonarcloud/project_links"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProjectLinksDataSource struct {
	p *sonarcloudProvider
}

var _ datasource.DataSource = (*ProjectLinksDataSource)(nil)

func NewProjectLinksDataSource() datasource.DataSource {
	return &ProjectLinksDataSource{}
}

func (d *ProjectLinksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_links"
}

func (d *ProjectLinksDataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This datasource retrieves the list of links for the given project.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"project_key": {
				Type:        types.StringType,
				Optional:    true,
				Description: "The key of the project.",
			},
			"links": {
				Computed:    true,
				Description: "The links of this project.",
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"id": {
						Type:        types.StringType,
						Computed:    true,
						Description: "ID of the link.",
					},
					"name": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The name the link.",
					},
					"type": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The type of the link.",
					},
					"url": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The url of the link.",
					},
				}),
			},
		},
	}, nil
}

func (d *ProjectLinksDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	d.p, resp.Diagnostics = toProvider(req.ProviderData)
	/*client, ok := req.ProviderData.(*sonarcloud.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *sonarcloud.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.p.client = client*/
}

func (d *ProjectLinksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DataProjectLinks
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := pl.SearchRequest{
		ProjectKey: config.ProjectKey.Value,
	}

	response, err := d.p.client.ProjectLinks.Search(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the project's links",
			fmt.Sprintf("The Search request returned an error: %+v", err),
		)
		return
	}

	links := make([]DataProjectLink, len(response.Links))
	for i, link := range response.Links {
		links[i] = DataProjectLink{
			Id:   types.String{Value: link.Id},
			Name: types.String{Value: link.Name},
			Type: types.String{Value: link.Type},
			Url:  types.String{Value: link.Url},
		}
	}

	result := DataProjectLinks{
		ID:         types.String{Value: config.ProjectKey.Value},
		ProjectKey: config.ProjectKey,
		Links:      links,
	}

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
