package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/user_tokens"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserTokenResource struct {
	p *sonarcloudProvider
}

func NewUserTokenResource() resource.Resource {
	return &UserTokenResource{}
}

func (*UserTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_token"
}

func (d *UserTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (*UserTokenResource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This resource manages the tokens for a user.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"login": {
				Type:        types.StringType,
				Required:    true,
				Description: "The login of the user to which the token should be added. This should be the same user as configured in the provider.",
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
			},
			"name": {
				Type:        types.StringType,
				Required:    true,
				Description: "The name of the token.",
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
			},
			"token": {
				Type:        types.StringType,
				Description: "The value of the generated token.",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}, nil
}

func (r UserTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan Token
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := user_tokens.GenerateRequest{
		Login: plan.Login.Value,
		Name:  plan.Name.Value,
	}

	res, err := r.p.client.UserTokens.Generate(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the user_token",
			fmt.Sprintf("The Generate request returned an error: %+v", err),
		)
		return
	}

	var result = Token{
		ID:    types.String{Value: res.Name},
		Login: types.String{Value: res.Login},
		Name:  types.String{Value: res.Name},
		Token: types.String{Value: res.Token},
	}
	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}

func (r UserTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Retrieve values from state
	var state Token
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := user_tokens.SearchRequest{
		Login: state.Login.Value,
	}

	response, err := r.p.client.UserTokens.Search(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the user_token",
			fmt.Sprintf("The Search request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists the list of retrieved resources
	if tokenExists(response, state.Name.Value) {
		// We cannot read the token value, so just write back the original state
		diags = resp.State.Set(ctx, state)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r UserTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// NOOP, we always need to recreate
}

func (r UserTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state Token
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := user_tokens.RevokeRequest{
		Login: state.Login.Value,
		Name:  state.Name.Value,
	}

	err := r.p.client.UserTokens.Revoke(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not delete the user_token",
			fmt.Sprintf("The Revoke request returned an error: %+v", err),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}
