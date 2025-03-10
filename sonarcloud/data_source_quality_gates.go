package sonarcloud

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kauppine/go-sonarcloud/sonarcloud/qualitygates"
)

type QualityGatesDataSource struct {
	p *sonarcloudProvider
}

func NewQualityGatesDataSource() datasource.DataSource {
	return &QualityGatesDataSource{}
}

func (*QualityGatesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quality_gates"
}

func (d *QualityGatesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d QualityGatesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This data source retrieves all Quality Gates for the configured organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The index of the Quality Gate",
				Computed:    true,
			},
			"quality_gates": schema.SetNestedAttribute{
				Computed:    true,
				Description: "A quality gate",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Id for Terraform backend",
							Computed:    true,
						},
						"gate_id": schema.Float64Attribute{
							Description: "Id created by SonarCloud",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Name of the Quality Gate",
							Computed:    true,
						},
						"is_default": schema.BoolAttribute{
							Description: "Is this the default Quality gate for this project?",
							Computed:    true,
						},
						"is_built_in": schema.BoolAttribute{
							Description: "Is this Quality gate built in?",
							Computed:    true,
						},
						"conditions": schema.SetNestedAttribute{
							Optional:    true,
							Computed:    true,
							Description: "The conditions of this quality gate.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.Float64Attribute{
										Description: "ID of the Condition.",
										Computed:    true,
									},
									"metric": schema.StringAttribute{
										Description: "The metric on which the condition is based. Must be one of: https://docs.sonarqube.org/latest/user-guide/metric-definitions/",
										Computed:    true,
										Validators: []validator.String{
											stringvalidator.OneOf("security_rating", "ncloc_language_distribution", "test_execution_time", "statements", "lines_to_cover", "quality_gate_details", "new_reliabillity_remediation_effort", "tests", "security_review_rating", "new_xxx_violations", "conditions_by_line", "new_violations", "ncloc", "duplicated_lines", "test_failures", "test_errors", "reopened_issues", "new_vulnerabilities", "duplicated_lines_density", "test_success_density", "sqale_debt_ratio", "security_hotspots_reviewed", "security_remediation_effort", "covered_conditions_by_line", "classes", "sqale_rating", "xxx_violations", "true_positive_issues", "violations", "new_security_review_rating", "new_security_remediation_effort", "vulnerabillities", "new_uncovered_conditions", "files", "branch_coverage_hits_data", "uncovered_lines", "comment_lines_density", "new_uncovered_lines", "complexty", "cognitive_complexity", "uncovered_conditions", "functions", "new_technical_debt", "new_coverage", "coverage", "new_branch_coverage", "confirmed_issues", "reliabillity_remediation_effort", "projects", "coverage_line_hits_data", "code_smells", "directories", "lines", "bugs", "line_coverage", "new_line_coverage", "reliability_rating", "duplicated_blocks", "branch_coverage", "new_code_smells", "new_sqale_debt_ratio", "open_issues", "sqale_index", "new_lines_to_cover", "comment_lines", "skipped_tests"),
										},
									},
									"op": schema.StringAttribute{
										Description: "Operation on which the metric is evaluated must be either: LT, GT",
										Optional:    true,
										Validators: []validator.String{
											stringvalidator.OneOf("LT", "GT"),
										},
									},
									"error": schema.StringAttribute{
										Description: "The value on which the condition errors.",
										Computed:    true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d QualityGatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var diags diag.Diagnostics

	request := qualitygates.ListRequest{}

	response, err := d.p.client.Qualitygates.List(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not read the Quality Gate",
			fmt.Sprintf("The List request returned an error: %+v", err),
		)
		return
	}

	result := QualityGates{}
	var allQualityGates []QualityGate
	for _, qualityGate := range response.Qualitygates {
		var allConditions []Condition
		for _, condition := range qualityGate.Conditions {
			allConditions = append(allConditions, Condition{
				Error:  types.StringValue(condition.Error),
				ID:     types.Float64Value(condition.Id),
				Metric: types.StringValue(condition.Metric),
				Op:     types.StringValue(condition.Op),
			})
		}
		allQualityGates = append(allQualityGates, QualityGate{
			ID:         types.StringValue(fmt.Sprintf("%d", int(qualityGate.Id))),
			GateId:     types.Float64Value(qualityGate.Id),
			IsBuiltIn:  types.BoolValue(qualityGate.IsBuiltIn),
			IsDefault:  types.BoolValue(qualityGate.IsDefault),
			Name:       types.StringValue(qualityGate.Name),
			Conditions: allConditions,
		})
	}
	result.QualityGates = allQualityGates
	result.ID = types.StringValue(d.p.organization)

	diags = resp.State.Set(ctx, result)

	resp.Diagnostics.Append(diags...)
}
