package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/qualitygates"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type QualityGateSelectionResource struct {
	p *sonarcloudProvider
}

func NewQualityGateSelectionResource() resource.Resource {
	return &QualityGateSelectionResource{}
}

func (*QualityGateSelectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quality_gate_selection"
}

func (d *QualityGateSelectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r QualityGateSelectionResource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This resource selects a quality gate for one or more projects",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:        types.StringType,
				Description: "The implicit ID of the resource",
				Computed:    true,
			},
			"gate_id": {
				Type:        types.StringType,
				Description: "The ID of the quality gate that is selected for the project(s).",
				Required:    true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
			},
			"project_keys": {
				Type:        types.SetType{ElemType: types.StringType},
				Description: "The Keys of the projects which have been selected on the referenced quality gate",
				Required:    true,
			},
		},
	}, nil
}

func (r QualityGateSelectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unkown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan Selection
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, s := range plan.ProjectKeys.Elements() {
		// Fill in api action struct for Quality Gates
		request := qualitygates.SelectRequest{
			GateId:       plan.GateId.ValueString(),
			ProjectKey:   s.(types.String).ValueString(),
			Organization: r.p.organization,
		}
		err := r.p.client.Qualitygates.Select(request)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not create Quality Gate Selection",
				fmt.Sprintf("The Select request returned an error: %+v", err),
			)
			return
		}
	}

	// Query for selection
	searchRequest := qualitygates.SearchRequest{
		GateId:       plan.GateId.ValueString(),
		Organization: r.p.organization,
	}

	res, err := r.p.client.Qualitygates.Search(searchRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read Quality Gate Selection",
			fmt.Sprintf("The Search request returned an error: %+v", err),
		)
		return
	}

	if result, ok := findSelection(res, plan.ProjectKeys.Elements()); ok {
		result.GateId = types.StringValue(plan.GateId.ValueString())
		result.ID = types.StringValue(plan.GateId.ValueString())
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.Diagnostics.AddError(
			"Could not find Quality Gate Selection",
			fmt.Sprintf("The findSelection function was unable to find the project keys: %+v in the response: %+v", plan.ProjectKeys.Elements(), res),
		)
		return
	}
}

func (r QualityGateSelectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Selection
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	searchRequest := qualitygates.SearchRequest{
		GateId:       state.GateId.ValueString(),
		Organization: r.p.organization,
	}
	res, err := r.p.client.Qualitygates.Search(searchRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not Read the Quality Gate Selection",
			fmt.Sprintf("The Search request returned an error: %+v", err),
		)
		return
	}
	if result, ok := findSelection(res, state.ProjectKeys.Elements()); ok {
		result.GateId = types.StringValue(state.GateId.ValueString())
		result.ID = types.StringValue(state.GateId.ValueString())
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r QualityGateSelectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state Selection
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan Selection
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sel, rem := diffSelection(state, plan)

	for _, s := range rem {
		deselectRequest := qualitygates.DeselectRequest{
			Organization: r.p.organization,
			ProjectKey:   s.(types.String).ValueString(),
		}
		err := r.p.client.Qualitygates.Deselect(deselectRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not Deselect the Quality Gate selection",
				fmt.Sprintf("The Deselect request returned an error: %+v", err),
			)
			return
		}
	}
	for _, s := range sel {
		selectRequest := qualitygates.SelectRequest{
			GateId:       state.GateId.ValueString(),
			Organization: r.p.organization,
			ProjectKey:   s.(types.String).ValueString(),
		}
		err := r.p.client.Qualitygates.Select(selectRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not Select the Quality Gate selection",
				fmt.Sprintf("The Select request returned an error: %+v", err),
			)
			return
		}
	}

	request := qualitygates.SearchRequest{
		GateId:       plan.GateId.ValueString(),
		Organization: r.p.organization,
	}
	res, err := r.p.client.Qualitygates.Search(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not Read the Quality Gate Selection",
			fmt.Sprintf("The Search request returned an error: %+v", err),
		)
		return
	}
	if result, ok := findSelection(res, plan.ProjectKeys.Elements()); ok {
		result.GateId = types.StringValue(state.GateId.ValueString())
		result.ID = types.StringValue(state.GateId.ValueString())
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.Diagnostics.AddError(
			"Could not find Quality Gate Selection",
			fmt.Sprintf("The findSelection function was unable to find the project keys: %+v in the response: %+v", plan.ProjectKeys.Elements(), res),
		)
		return
	}
}

func (r QualityGateSelectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Selection
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, s := range state.ProjectKeys.Elements() {
		request := qualitygates.DeselectRequest{
			Organization: r.p.organization,
			ProjectKey:   s.(types.String).ValueString(),
		}
		err := r.p.client.Qualitygates.Deselect(request)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not Deselect the Quality Gate Selection",
				fmt.Sprintf("The Deselect request returned an error: %+v", err),
			)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func diffSelection(state, plan Selection) (sel, rem []attr.Value) {
	for _, old := range state.ProjectKeys.Elements() {
		// assume that old is a string
		if !containSelection(plan.ProjectKeys.Elements(), old.(types.String).ValueString()) {
			rem = append(rem, types.StringValue(old.(types.String).ValueString()))
		}
	}
	for _, new := range plan.ProjectKeys.Elements() {
		// assume that new is a string
		if !containSelection(state.ProjectKeys.Elements(), new.(types.String).ValueString()) {
			sel = append(sel, types.StringValue(new.(types.String).ValueString()))
		}
	}

	return sel, rem
}

// Check if a condition is contained in a condition list
func containSelection(list []attr.Value, item string) bool {
	for _, c := range list {
		if c.Equal(types.StringValue(item)) {
			return true
		}
	}
	return false
}
