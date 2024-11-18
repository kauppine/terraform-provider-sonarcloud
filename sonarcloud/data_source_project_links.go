package sonarcloud

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pl "github.com/kauppine/go-sonarcloud/sonarcloud/project_links"
)

type ProjectLinksDataSource struct {
	p *sonarcloudProvider
}

var _ datasource.DataSource = (*ProjectLinksDataSource)(nil)
var _ datasource.DataSourceWithConfigure = &ProjectLinksDataSource{}

func NewProjectLinksDataSource() datasource.DataSource {
	return &ProjectLinksDataSource{}
}

func (d *ProjectLinksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_links"
}

func (d *ProjectLinksDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This datasource retrieves the list of links for the given project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"project_key": schema.StringAttribute{
				Optional:    true,
				Description: "The key of the project.",
			},
			"links": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The links of this project.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "ID of the link.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name the link.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The type of the link.",
						},
						"url": schema.StringAttribute{
							Computed:    true,
							Description: "The url of the link.",
						},
					},
				},
			},
		},
	}
}

func (d *ProjectLinksDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProjectLinksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DataProjectLinks
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := pl.SearchRequest{
		ProjectKey: config.ProjectKey.ValueString(),
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
			Id:   types.StringValue(link.Id),
			Name: types.StringValue(link.Name),
			Type: types.StringValue(link.Type),
			Url:  types.StringValue(link.Url),
		}
	}

	result := DataProjectLinks{
		ID:         types.StringValue(config.ProjectKey.ValueString()),
		ProjectKey: config.ProjectKey,
		Links:      links,
	}

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
