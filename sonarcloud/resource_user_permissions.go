package sonarcloud

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ArgonGlow/go-sonarcloud/sonarcloud"
	"github.com/ArgonGlow/go-sonarcloud/sonarcloud/permissions"
	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserPermissionsResource struct {
	p *sonarcloudProvider
}

func NewUserPermissionsResource() resource.Resource {
	return &UserPermissionsResource{}
}

func (*UserPermissionsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_permissions"
}

func (d *UserPermissionsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r UserPermissionsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This resource manages the permissions of a user for the whole organization or a specific project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The implicit ID of the resource.",
				Computed:    true,
			},
			"project_key": schema.StringAttribute{
				Optional:    true,
				Description: "The key of the project to restrict the permissions to.",
			},
			"login": schema.StringAttribute{
				Required:    true,
				Description: "The login of the user to set the permissions for.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the user.",
			},
			"permissions": schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "List of permissions to grant." +
					" Available global permissions: [`admin`, `profileadmin`, `gateadmin`, `scan`, `provisioning`]." +
					" Available project permissions: ['admin`, `scan`, `codeviewer`, `issueadmin`, `securityhotspotadmin`, `user`].",
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(
						// Global permissions
						"admin",
						"profileadmin",
						"gateadmin",
						"scan",
						"provisioning",
						// Project permissions
						// Note: admin and scan are project permissions as well
						"codeviewer",
						"issueadmin",
						"securityhotspotadmin",
						"user",
					)),
				},
			},
			"avatar": schema.StringAttribute{
				Computed:    true,
				Description: "The avatar ID of the user.",
			},
		},
	}
}

func (r UserPermissionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unkown value from another resource. "+
				"This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan UserPermissions
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add permissions one by one
	wg := sync.WaitGroup{}
	for _, elem := range plan.Permissions.Elements() {
		permission := elem.(types.String).ValueString()

		wg.Add(1)
		go func() {
			defer wg.Done()

			request := permissions.AddUserRequest{
				Login:        plan.Login.ValueString(),
				Permission:   permission,
				ProjectKey:   plan.ProjectKey.ValueString(),
				Organization: r.p.organization,
			}
			if err := r.p.client.Permissions.AddUser(request); err != nil {
				resp.Diagnostics.AddError(
					"Could not add user permissions",
					fmt.Sprintf("The AddUser request returned an error: %+v", err),
				)
				return
			}
		}()
	}

	// Async wait for all requests to be done
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()

	// Set ID on success and return error diag on timeout
	select {
	case <-c:
	case <-time.After(30 * time.Second):
		resp.Diagnostics.AddError("Could not set user user permissions",
			"The requests to set the permissions timed out.",
		)
	}

	plannedPermissions := make([]string, len(plan.Permissions.Elements()))
	diags = plan.Permissions.ElementsAs(ctx, &plannedPermissions, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	backoffConfig := defaultBackoffConfig()

	user, err := backoff.RetryWithData(
		func() (*UserPermissions, error) {
			user, err := findUserWithPermissionsSet(r.p.client, plan.Login.ValueString(), plan.ProjectKey.ValueString(), plan.Permissions)
			return user, err
		}, backoffConfig)

	if err != nil {
		resp.Diagnostics.AddError(
			"Could not find the user with the planned permissions",
			fmt.Sprintf("The findUserWithPermissionsSet call returned an error: %+v ", err),
		)
	} else {
		diags = resp.State.Set(ctx, user)
		resp.Diagnostics.Append(diags...)
	}
}

func (r UserPermissionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserPermissions
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query for permissions
	searchRequest := UserPermissionsSearchRequest{ProjectKey: state.ProjectKey.ValueString()}
	users, err := sonarcloud.GetAll[UserPermissionsSearchRequest, UserPermissionsSearchResponseUser](r.p.client, "/permissions/users", searchRequest, "users")
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get user permissions",
			fmt.Sprintf("The request returned an error: %+v", err),
		)
		return
	}

	if user, ok := findUser(users, state.Login.ValueString()); ok {
		permissionsElems := make([]attr.Value, len(user.Permissions))

		for i, permission := range user.Permissions {
			permissionsElems[i] = types.StringValue(permission)
		}

		result := UserPermissions{
			ID:          types.StringValue(state.ProjectKey.ValueString() + "-" + state.Login.ValueString()),
			ProjectKey:  state.ProjectKey,
			Login:       types.StringValue(user.Login),
			Name:        types.StringValue(user.Name),
			Permissions: types.SetValueMust(types.StringType, permissionsElems),
			Avatar:      types.StringValue(user.Avatar),
		}
		diags = resp.State.Set(ctx, result)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.State.RemoveResource(ctx)
	}
}

func (r UserPermissionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state UserPermissions
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan UserPermissions
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	toAdd, toRemove := diffAttrSets(state.Permissions, plan.Permissions)

	for _, remove := range toRemove {
		removeRequest := permissions.RemoveUserRequest{
			Login:        state.Login.ValueString(),
			Organization: r.p.organization,
			Permission:   remove.(types.String).ValueString(),
			ProjectKey:   state.ProjectKey.ValueString(),
		}
		err := r.p.client.Permissions.RemoveUser(removeRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not remove the permission",
				fmt.Sprintf("The RemoveUser request returned an error: %+v", err),
			)
			return
		}
	}
	for _, add := range toAdd {
		addRequest := permissions.AddUserRequest{
			Login:        plan.Login.ValueString(),
			Permission:   add.(types.String).ValueString(),
			ProjectKey:   plan.ProjectKey.ValueString(),
			Organization: r.p.organization,
		}
		if err := r.p.client.Permissions.AddUser(addRequest); err != nil {
			resp.Diagnostics.AddError(
				"Could not add the user permission",
				fmt.Sprintf("The AddUser request returned an error: %+v", err),
			)
			return
		}
	}

	plannedPermissions := make([]string, len(plan.Permissions.Elements()))
	diags = plan.Permissions.ElementsAs(ctx, &plannedPermissions, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	backoffConfig := defaultBackoffConfig()

	user, err := backoff.RetryWithData(
		func() (*UserPermissions, error) {
			return findUserWithPermissionsSet(r.p.client, plan.Login.ValueString(), plan.ProjectKey.ValueString(), plan.Permissions)
		}, backoffConfig)

	if err != nil {
		resp.Diagnostics.AddError(
			"Could not find the user with the planned permissions",
			fmt.Sprintf("The findUserWithPermissionsSet call returned an error: %+v ", err),
		)
	} else {
		diags = resp.State.Set(ctx, user)
		resp.Diagnostics.Append(diags...)
	}
}

func (r UserPermissionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserPermissions
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, remove := range state.Permissions.Elements() {
		removeRequest := permissions.RemoveUserRequest{
			Login:        state.Login.ValueString(),
			Organization: r.p.organization,
			Permission:   remove.(types.String).ValueString(),
			ProjectKey:   state.ProjectKey.ValueString(),
		}
		err := r.p.client.Permissions.RemoveUser(removeRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Could not remove the user permission",
				fmt.Sprintf("The RemoveUser request returned an error: %+v", err),
			)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r UserPermissionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")
	if len(idParts) < 1 || len(idParts) > 2 || idParts[0] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: login OR login,project_key. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("login"), idParts[0])...)
	if len(idParts) == 2 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_key"), idParts[1])...)
	}
}

type UserPermissionsSearchRequest struct {
	ProjectKey string
}

type UserPermissionsSearchResponseUser struct {
	Id          string   `json:"id,omitempty"`
	Login       string   `json:"login,omitempty"`
	Name        string   `json:"name,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Avatar      string   `json:"avatar,omitempty"`
}

// findUserWithPermissionsSet tries to find a user with the given login and the expected permissions
func findUserWithPermissionsSet(client *sonarcloud.Client, login, projectKey string, expectedPermissions types.Set) (*UserPermissions, error) {
	searchRequest := UserGroupPermissionsSearchRequest{ProjectKey: projectKey}
	users, err := sonarcloud.GetAll[UserGroupPermissionsSearchRequest, UserPermissionsSearchResponseUser](client, "/permissions/users", searchRequest, "users")
	if err != nil {
		return nil, err
	}

	user, ok := findUser(users, login)
	if !ok {
		return nil, fmt.Errorf("user not found in response (login='%s',projectKey='%s')", login, projectKey)
	}

	permissionsElems := make([]attr.Value, len(user.Permissions))
	for i, permission := range user.Permissions {
		permissionsElems[i] = types.StringValue(permission)
	}

	foundPermissions, _ := types.SetValue(types.StringType, permissionsElems)

	if !foundPermissions.Equal(expectedPermissions) {
		return nil, fmt.Errorf("the returned permissions do not match the expected permissions (login='%s',projectKey='%s, expected='%v', got='%v')",
			login,
			projectKey,
			expectedPermissions,
			foundPermissions)
	}

	return &UserPermissions{
		ID:          types.StringValue(projectKey + "-" + login),
		ProjectKey:  types.StringValue(projectKey),
		Login:       types.StringValue(user.Login),
		Name:        types.StringValue(user.Name),
		Permissions: foundPermissions,
		Avatar:      types.StringValue(user.Avatar),
	}, nil

}

// findUser returns the user with the given login, if it exists
func findUser(users []UserPermissionsSearchResponseUser, login string) (*UserPermissionsSearchResponseUser, bool) {
	for _, user := range users {
		if user.Login == login {
			return &user, true
		}
	}
	return nil, false
}
