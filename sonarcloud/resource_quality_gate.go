package sonarcloud

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/qualitygates"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type QualityGateResource struct {
	p *sonarcloudProvider
}

func NewQualityGateResource() resource.Resource {
	return &QualityGateResource{}
}

func (*QualityGateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quality_gate"
}

func (d *QualityGateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r QualityGateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This resource manages a Quality Gate",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Implicit Terraform ID",
				Computed:    true,
			},
			"gate_id": schema.Float64Attribute{
				Description: "Id computed by SonarCloud servers",
				Computed:    true,
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the Quality Gate.",
				Required:    true,
			},
			"is_built_in": schema.BoolAttribute{
				Description: "Defines whether the quality gate is built in.",
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_default": schema.BoolAttribute{
				Description: "Defines whether the quality gate is the default gate for an organization. **WARNING**: Must be assigned to one quality gate per organization at all times.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"conditions": schema.SetNestedAttribute{
				Optional:    true,
				Description: "The conditions of this quality gate. Please query https://sonarcloud.io/api/metrics/search for an up-to-date list of conditions.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Float64Attribute{
							Description: "Index/ID of the Condition.",
							Computed:    true,
						},
						"metric": schema.StringAttribute{
							Description: "The metric on which the condition is based.",
							Required:    true,
						},
						"op": schema.StringAttribute{
							Description: "Operation on which the metric is evaluated must be either: LT, GT.",
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.OneOf("LT", "GT"),
							},
						},
						"error": schema.StringAttribute{
							Description: "The value on which the condition errors.",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

func (r QualityGateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unkown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan QualityGate
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct for Quality Gates
	request := qualitygates.CreateRequest{
		Name:         plan.Name.ValueString(),
		Organization: r.p.organization,
	}

	res, err := r.p.client.Qualitygates.Create(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the Quality Gate",
			fmt.Sprintf("The Quality Gate create request returned an error: %+v", err),
		)
		return
	}

	var result = QualityGate{
		ID:     types.StringValue(fmt.Sprintf("%d", int(res.Id))),
		GateId: types.Float64Value(res.Id),
		Name:   types.StringValue(res.Name),
	}

	if plan.IsDefault.ValueBool() {
		setDefualtRequest := qualitygates.SetAsDefaultRequest{
			Id:           fmt.Sprintf("%d", int(result.GateId.ValueFloat64())),
			Organization: r.p.organization,
		}
		err := r.p.client.Qualitygates.SetAsDefault(setDefualtRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not set Quality Gate as default",
				fmt.Sprintf("The Quality Gate SetAsDefault request returned an error: %+v", err),
			)
		}
	}

	conditionRequests := qualitygates.CreateConditionRequest{}
	for _, conditionPlan := range plan.Conditions {
		conditionRequests = qualitygates.CreateConditionRequest{
			Error:        conditionPlan.Error.ValueString(),
			GateId:       fmt.Sprintf("%d", int(result.GateId.ValueFloat64())),
			Metric:       conditionPlan.Metric.ValueString(),
			Op:           conditionPlan.Op.ValueString(),
			Organization: r.p.organization,
		}
		res, err := r.p.client.Qualitygates.CreateCondition(conditionRequests)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not create a Condition",
				fmt.Sprintf("The Condition Create Request returned an error: %+v", err),
			)
			return
		}
		// didn't implement warning
		result.Conditions = append(result.Conditions, Condition{
			Error:  types.StringValue(res.Error),
			ID:     types.Float64Value(res.Id),
			Metric: types.StringValue(res.Metric),
			Op:     types.StringValue(res.Op),
		})
	}

	// Actions are not returned with create request, so we need to query for them
	listRequest := qualitygates.ListRequest{
		Organization: r.p.organization,
	}

	listRes, err := r.p.client.Qualitygates.List(listRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the Quality Gate",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	if createdQualityGate, ok := findQualityGate(listRes, result.Name.ValueString()); ok {
		result.IsBuiltIn = createdQualityGate.IsBuiltIn
		result.IsDefault = createdQualityGate.IsDefault
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
}

func (r QualityGateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	//Retrieve values from state
	var state QualityGate
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in api action struct
	request := qualitygates.ListRequest{
		Organization: r.p.organization,
	}

	response, err := r.p.client.Qualitygates.List(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the Quality Gate(s)",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	// Check if the resource exists in the list of retrieved resources
	if result, ok := findQualityGate(response, state.Name.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

// Some good examples of update functions for SetNestedAttributes:
// https://github.com/vercel/terraform-provider-vercel/blob/b38f0abb6774bf2b0314bc94808d497f2e7b9e50/vercel/resource_project.go
// https://github.com/adnsio/terraform-provider-k0s/blob/c8db5204e70e15484973d5680fe14ed184e719ef/internal/provider/cluster_resource.go#L366
// https://github.com/devopsarr/terraform-provider-sonarr/blob/078ba51ca03a7782af5fbaaf48f6ebd15284116c/internal/provider/quality_profile_resource.go (DOUBLE NESTED!!! :O)
// Thanks to those who wrote the above resources, they really helped me (Arnav Bhutani @Bhutania) out :)
func (r QualityGateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	//retrieve values from state
	var state QualityGate
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from plan
	var plan QualityGate
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diffName(state, plan) {
		request := qualitygates.RenameRequest{
			Id:           fmt.Sprintf("%d", int(state.GateId.ValueFloat64())),
			Name:         plan.Name.ValueString(),
			Organization: r.p.organization,
		}

		err := r.p.client.Qualitygates.Rename(request)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not update Quality Gate Name.",
				fmt.Sprintf("The Rename request returned an error: %+v", err),
			)
			return
		}
	}

	if diffDefault(state, plan) {
		if plan.IsDefault.Equal(types.BoolValue(true)) {
			request := qualitygates.SetAsDefaultRequest{
				Id:           fmt.Sprintf("%d", int(state.GateId.ValueFloat64())),
				Organization: r.p.organization,
			}
			err := r.p.client.Qualitygates.SetAsDefault(request)
			if err != nil {
				resp.Diagnostics.AddError(
					"Could not set Quality Gate as Default.",
					fmt.Sprintf("The SetAsDefault request returned an error %+v", err),
				)
				return
			}
		}
		// Hard coded default present in all repositories (Sonar way)
		// This assumes that the Sonar way default quality gate will
		// never change its ID and remain the default forever.
		if plan.IsDefault.Equal(types.BoolValue(false)) {
			request := qualitygates.SetAsDefaultRequest{
				Id:           "9",
				Organization: r.p.organization,
			}
			err := r.p.client.Qualitygates.SetAsDefault(request)
			if err != nil {
				resp.Diagnostics.AddError(
					"Could not set `Sonar Way` quality gate to default",
					fmt.Sprintf("The SetAsDefault request returned an error %+v", err),
				)
			}
		}
	}

	toCreate, toUpdate, toRemove := diffConditions(state.Conditions, plan.Conditions)

	if len(toUpdate) > 0 {
		for _, c := range toUpdate {
			request := qualitygates.UpdateConditionRequest{
				Error:        c.Error.ValueString(),
				Id:           fmt.Sprintf("%d", int(c.ID.ValueFloat64())),
				Metric:       c.Metric.ValueString(),
				Op:           c.Op.ValueString(),
				Organization: r.p.organization,
			}

			err := r.p.client.Qualitygates.UpdateCondition(request)
			if err != nil {
				resp.Diagnostics.AddError(
					"Could not update QualityGate condition",
					fmt.Sprintf("The UpdateCondition request returned an error %+v", err),
				)
				return
			}
		}
	}
	if len(toCreate) > 0 {
		for _, c := range toCreate {
			request := qualitygates.CreateConditionRequest{
				GateId:       fmt.Sprintf("%d", int(state.GateId.ValueFloat64())),
				Error:        c.Error.ValueString(),
				Metric:       c.Metric.ValueString(),
				Op:           c.Op.ValueString(),
				Organization: r.p.organization,
			}
			_, err := r.p.client.Qualitygates.CreateCondition(request)
			if err != nil {
				resp.Diagnostics.AddError(
					"Could not create QualityGate condition",
					fmt.Sprintf("The CreateCondition request returned an error %+v", err),
				)
				return
			}
		}
	}
	if len(toRemove) > 0 {
		for _, c := range toRemove {
			request := qualitygates.DeleteConditionRequest{
				Id:           fmt.Sprintf("%d", int(c.ID.ValueFloat64())),
				Organization: r.p.organization,
			}
			err := r.p.client.Qualitygates.DeleteCondition(request)
			if err != nil {
				resp.Diagnostics.AddError(
					"Could not delete QualityGate condition",
					fmt.Sprintf("The DeleteCondition request returned an error %+v", err),
				)
				return
			}
		}
	}
	// There aren't any return values for non-create operations.
	listRequest := qualitygates.ListRequest{
		Organization: r.p.organization,
	}

	response, err := r.p.client.Qualitygates.List(listRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the Quality Gate",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	if result, ok := findQualityGate(response, plan.Name.ValueString()); ok {
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	}
}

func (r QualityGateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state QualityGate
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Hard coded default present in all repositories (Sonar way)
	// This assumes that the Sonar way default quality gate will
	// never change its ID and remain the default forever.
	if state.IsDefault.Equal(types.BoolValue(true)) {
		request := qualitygates.SetAsDefaultRequest{
			Id:           "9",
			Organization: r.p.organization,
		}
		err := r.p.client.Qualitygates.SetAsDefault(request)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not reset Organization's default quality gate pre-delete",
				fmt.Sprintf("The SetAsDefault request returned an error: %+v", err),
			)
		}
	}

	request := qualitygates.DestroyRequest{
		Id:           fmt.Sprintf("%d", int(state.GateId.ValueFloat64())),
		Organization: r.p.organization,
	}

	err := r.p.client.Qualitygates.Destroy(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not destroy the quality gate",
			fmt.Sprintf("The Destroy request returned an error: %+v", err),
		)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r QualityGateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// Check if quality Gate name is the same
func diffName(old, new QualityGate) bool {
	if old.Name.Equal(new.Name) {
		return false
	}
	return true
}

// Check if a Quality Gate has been set to default
func diffDefault(old, new QualityGate) bool {
	if old.IsDefault.Equal(new.IsDefault) {
		return false
	}
	return true
}

// Check if Quality Gate Conditions are different
func diffConditions(old, new []Condition) (create, update, remove []Condition) {
	create = []Condition{}
	remove = []Condition{}
	update = []Condition{}

	for _, c := range new {
		if !containsCondition(old, c) {
			create = append(create, c)
		} else {
			update = append(update, c)
		}
	}
	for _, c := range old {
		if !containsCondition(new, c) {
			remove = append(remove, c)
		}
	}

	return create, update, remove
}

// Check if a condition is contained in a condition list
func containsCondition(list []Condition, item Condition) bool {
	for _, c := range list {
		if c.Metric.Equal(item.Metric) {
			return true
		}
	}
	return false
}
