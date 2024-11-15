package sonarcloud

import (
	"context"
	"fmt"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/qualitygates"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dataSourceQualityGateType struct{}

func (d dataSourceQualityGateType) GetSchema(__ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: "This Data Source retrieves a single Quality Gate for the configured Organization.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:        types.StringType,
				Description: "Id for Terraform backend",
				Computed:    true,
			},
			"gate_id": {
				Type:        types.Float64Type,
				Description: "Id created by SonarCloud",
				Computed:    true,
			},
			"name": {
				Type:        types.StringType,
				Description: "Name of the Quality Gate",
				Required:    true,
			},
			"is_default": {
				Type:        types.BoolType,
				Description: "Is this the default Quality gate for this project?",
				Computed:    true,
			},
			"is_built_in": {
				Type:        types.BoolType,
				Description: "Is this Quality gate built in?",
				Computed:    true,
			},
			"conditions": {
				Optional:    true,
				Computed:    true,
				Description: "The conditions of this quality gate.",
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"id": {
						Type:        types.Float64Type,
						Description: "ID of the Condition.",
						Computed:    true,
					},
					"metric": {
						Type:        types.StringType,
						Description: "The metric on which the condition is based. Must be one of: https://docs.sonarqube.org/latest/user-guide/metric-definitions/",
						Computed:    true,
						Validators: []tfsdk.AttributeValidator{
							allowedOptions("security_rating", "ncloc_language_distribution", "test_execution_time", "statements", "lines_to_cover", "quality_gate_details", "new_reliabillity_remediation_effort", "tests", "security_review_rating", "new_xxx_violations", "conditions_by_line", "new_violations", "ncloc", "duplicated_lines", "test_failures", "test_errors", "reopened_issues", "new_vulnerabilities", "duplicated_lines_density", "test_success_density", "sqale_debt_ratio", "security_hotspots_reviewed", "security_remediation_effort", "covered_conditions_by_line", "classes", "sqale_rating", "xxx_violations", "true_positive_issues", "violations", "new_security_review_rating", "new_security_remediation_effort", "vulnerabillities", "new_uncovered_conditions", "files", "branch_coverage_hits_data", "uncovered_lines", "comment_lines_density", "new_uncovered_lines", "complexty", "cognitive_complexity", "uncovered_conditions", "functions", "new_technical_debt", "new_coverage", "coverage", "new_branch_coverage", "confirmed_issues", "reliabillity_remediation_effort", "projects", "coverage_line_hits_data", "code_smells", "directories", "lines", "bugs", "line_coverage", "new_line_coverage", "reliability_rating", "duplicated_blocks", "branch_coverage", "new_code_smells", "new_sqale_debt_ratio", "open_issues", "sqale_index", "new_lines_to_cover", "comment_lines", "skipped_tests"),
						},
					},
					"op": {
						Type:        types.StringType,
						Description: "Operation on which the metric is evaluated must be either: LT, GT",
						Optional:    true,
						Validators: []tfsdk.AttributeValidator{
							allowedOptions("LT", "GT"),
						},
					},
					"error": {
						Type:        types.StringType,
						Description: "The value on which the condition errors.",
						Computed:    true,
					},
				}),
			},
		},
	}, nil
}

func (d dataSourceQualityGateType) NewDataSource(_ context.Context, p provider.Provider) (datasource.DataSource, diag.Diagnostics) {
	return dataSourceQualityGate{
		p: *(p.(*sonarcloudProvider)),
	}, nil
}

type dataSourceQualityGate struct {
	p sonarcloudProvider
}

func (d dataSourceQualityGate) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config QualityGate
	diags := req.Config.Get(ctx, &config)
	if resp.Diagnostics.HasError() {
		return
	}

	request := qualitygates.ListRequest{}

	response, err := d.p.client.Qualitygates.List(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the Quality Gate",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	result := QualityGate{}
	for _, qualityGate := range response.Qualitygates {
		if qualityGate.Name == config.Name.Value {
			for _, condition := range qualityGate.Conditions {
				result.Conditions = append(result.Conditions, Condition{
					Error:  types.String{Value: condition.Error},
					ID:     types.Float64{Value: condition.Id},
					Metric: types.String{Value: condition.Metric},
					Op:     types.String{Value: condition.Op},
				})
			}
			result.ID = types.String{Value: fmt.Sprintf("%d", int(qualityGate.Id))}
			result.GateId = types.Float64{Value: qualityGate.Id}
			result.Name = types.String{Value: qualityGate.Name}
			result.IsDefault = types.Bool{Value: qualityGate.IsDefault}
			result.IsBuiltIn = types.Bool{Value: qualityGate.IsBuiltIn}

		}
	}

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
