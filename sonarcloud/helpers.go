package sonarcloud

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/kauppine/go-sonarcloud/sonarcloud/project_branches"
	"github.com/kauppine/go-sonarcloud/sonarcloud/projects"
	"github.com/kauppine/go-sonarcloud/sonarcloud/qualitygates"
	"github.com/kauppine/go-sonarcloud/sonarcloud/user_groups"
	"github.com/kauppine/go-sonarcloud/sonarcloud/user_tokens"
)

// changedAttrs returns a map where the keys are the names of all the attributes that were changed
// Note that the name is not the full path, but only the AttributeName of the last path step.
func changedAttrs(req resource.UpdateRequest, diags diag.Diagnostics) map[string]struct{} {
	diffs, err := req.Plan.Raw.Diff(req.State.Raw)
	if err != nil {
		diags.AddError(
			"Could not diff plan with state",
			"This should not happen and is an error in the provider.",
		)
	}

	changes := make(map[string]struct{})
	for _, diff := range diffs {
		steps := diff.Path.Steps()
		index := len(steps) - 1

		if !diff.Value1.Equal(*diff.Value2) {
			attr := steps[index].(tftypes.AttributeName)
			changes[string(attr)] = struct{}{}
		}
	}
	return changes
}

// findGroup returns the group with the given name if it exists in the response
func findGroup(response *user_groups.SearchResponseAll, name string) (Group, bool) {
	var result Group
	ok := false
	for _, g := range response.Groups {
		if g.Name == name {
			result = Group{
				ID:           types.StringValue(big.NewFloat(g.Id).String()),
				Default:      types.BoolValue(g.Default),
				Description:  types.StringValue(g.Description),
				MembersCount: types.NumberValue(big.NewFloat(g.MembersCount)),
				Name:         types.StringValue(g.Name),
			}
			ok = true
			break
		}
	}
	return result, ok
}

// findGroup returns the group member with the given login if it exists in the response
func findGroupMember(response *user_groups.UsersResponseAll, group string, login string) (GroupMember, bool) {
	var result GroupMember
	ok := false
	for _, u := range response.Users {
		if u.Login == login {
			result = GroupMember{
				Group: types.StringValue(group),
				Login: types.StringValue(login),
			}
			ok = true
			break
		}
	}
	return result, ok
}

// tokenExists returns whether a token with the given name exists in the response
func tokenExists(response *user_tokens.SearchResponse, name string) bool {
	for _, t := range response.UserTokens {
		if t.Name == name {
			return true
		}
	}
	return false
}

// findProject returns the project with the given key if it exists in the response
func findProject(response *projects.SearchResponseAll, key string) (Project, bool) {
	var result Project
	ok := false
	for _, p := range response.Components {
		if p.Key == key {
			result = Project{
				ID:         types.StringValue(p.Key),
				Name:       types.StringValue(p.Name),
				Key:        types.StringValue(p.Key),
				Visibility: types.StringValue(p.Visibility),
			}
			ok = true
			break
		}
	}
	return result, ok
}

// findProjectMainBranch returns the main branch with the given name if it exists in the response
func findProjectMainBranch(response *project_branches.ListResponse, name, projectKey string) (ProjectMainBranch, bool) {
	var result ProjectMainBranch
	ok := false
	for _, p := range response.Branches {
		if p.Name == name && p.IsMain {
			result = ProjectMainBranch{
				ID:         types.StringValue(p.Name),
				Name:       types.StringValue(p.Name),
				ProjectKey: types.StringValue(projectKey),
			}
			ok = true
			break
		}
	}
	return result, ok
}

// findQualityGate returns the quality gate with the given name if it exists in a response
func findQualityGate(response *qualitygates.ListResponse, name string) (QualityGate, bool) {
	var result QualityGate
	ok := false
	for _, q := range response.Qualitygates {
		if q.Name == name {
			result = QualityGate{
				ID:        types.StringValue(fmt.Sprintf("%d", int(q.Id))),
				GateId:    types.Float64Value(q.Id),
				Name:      types.StringValue(q.Name),
				IsBuiltIn: types.BoolValue(q.IsBuiltIn),
				IsDefault: types.BoolValue(q.IsDefault),
			}
			for _, c := range q.Conditions {
				result.Conditions = append(result.Conditions, Condition{
					Error:  types.StringValue(c.Error),
					ID:     types.Float64Value(c.Id),
					Metric: types.StringValue(c.Metric),
					Op:     types.StringValue(c.Op),
				})
			}
			ok = true
			break
		}
	}
	return result, ok
}

// findSelection returns a Selection{} struct with the given project keys if they exist in a response
// this can be sped up using hashmaps, but I didn't feel like introducing a new dependency/taking code from somewhere.
// Ex library: https://pkg.go.dev/github.com/juliangruber/go-intersect/v2
func findSelection(response *qualitygates.SearchResponse, keys []attr.Value) (Selection, bool) {
	projectKeys := make([]attr.Value, 0)
	ok := true
	for _, k := range keys {
		ok = false
		for _, s := range response.Results {
			if k.Equal(types.StringValue(s.Key)) {
				projectKeys = append(projectKeys, types.StringValue(strings.Trim(s.Key, "\"")))
				ok = true
				break
			}
		}
		if !ok {
			break
		}
	}
	return Selection{
		ProjectKeys: types.SetValueMust(types.StringType, projectKeys),
	}, ok
}

// terraformListString returns the list of items in terraform list notation
func terraformListString(items []string) string {
	return fmt.Sprintf(`["%s"]`, strings.Join(items, `","`))
}

// defaultBackendConfig returns an exponential backoff with a timeout of 30 seconds instead of the module's default of 15 minutes
func defaultBackoffConfig() *backoff.ExponentialBackOff {
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxInterval = 10 * time.Second
	backoffConfig.MaxElapsedTime = 30 * time.Second
	backoffConfig.InitialInterval = 250 * time.Millisecond
	return backoffConfig
}

// stringAttributesContain checks if the given string is found in the list of attributes
func stringAttributesContain(haystack []attr.Value, needle string) bool {
	for _, v := range haystack {
		if v.Equal(types.StringValue(needle)) {
			return true
		}
	}
	return false
}

// diffAttrSets returns the additions and deletions needed to get from the set we have, to the set we want
func diffAttrSets(haves, wants types.Set) (toAdd, toRemove []attr.Value) {
	for _, have := range haves.Elements() {
		if !stringAttributesContain(wants.Elements(), have.(types.String).ValueString()) {
			toRemove = append(toRemove, types.StringValue(have.(types.String).ValueString()))
		}
	}
	for _, want := range wants.Elements() {
		if !stringAttributesContain(haves.Elements(), want.(types.String).ValueString()) {
			toAdd = append(toAdd, types.StringValue(want.(types.String).ValueString()))
		}
	}

	return toAdd, toRemove
}
