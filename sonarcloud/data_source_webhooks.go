package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/webhooks"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type WebhooksDataSource struct {
	p sonarcloudProvider
}

func NewWebhooksDataSource() datasource.DataSource {
	return &WebhooksDataSource{}
}

func (*WebhooksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhooks"
}

func (d WebhooksDataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This datasource retrieves the list of webhooks for a project or the organization.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"project": {
				Type:        types.StringType,
				Optional:    true,
				Description: "The key of the project. If empty, the webhooks of the organization are returned.",
			},
			"webhooks": {
				Computed:    true,
				Description: "The webhooks of this project or organization.",
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"key": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The key of the webhook.",
					},
					"name": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The name the webhook.",
					},
					"url": {
						Type:        types.StringType,
						Computed:    true,
						Description: "The url of the webhook.",
					},
					"has_secret": {
						Type:        types.BoolType,
						Computed:    true,
						Description: "Whether the webhook has a secret.",
					},
				}),
			},
		},
	}, nil
}

func (d WebhooksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DataWebhooks
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := webhooks.ListRequest{
		Organization: d.p.organization,
		Project:      config.Project.Value,
	}

	response, err := d.p.client.Webhooks.List(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the webhooks",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	hooks := make([]DataWebhook, len(response.Webhooks))
	for i, webhook := range response.Webhooks {
		hooks[i] = DataWebhook{
			Key:       types.String{Value: webhook.Key},
			Name:      types.String{Value: webhook.Name},
			HasSecret: types.Bool{Value: webhook.HasSecret},
			Url:       types.String{Value: webhook.Url},
		}
	}

	result := DataWebhooks{
		ID:       types.String{Value: fmt.Sprintf("%s-%s", d.p.organization, config.Project.Value)},
		Project:  config.Project,
		Webhooks: hooks,
	}

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
