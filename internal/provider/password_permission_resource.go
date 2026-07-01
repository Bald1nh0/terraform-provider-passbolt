package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

var (
	_ resource.Resource                     = &passwordPermissionResource{}
	_ resource.ResourceWithConfigure        = &passwordPermissionResource{}
	_ resource.ResourceWithConfigValidators = &passwordPermissionResource{}
	_ resource.ResourceWithImportState      = &passwordPermissionResource{}
)

const (
	passwordPermissionAROGroup = "Group"
	passwordPermissionAROUser  = "User"

	passwordPermissionImportKindGroup = "group"
	passwordPermissionImportKindUser  = "user"
)

var errPasswordPermissionNotFound = errors.New("password permission not found")

// NewPasswordPermissionResource returns a Terraform resource for managing Passbolt password permissions.
func NewPasswordPermissionResource() resource.Resource {
	return &passwordPermissionResource{}
}

type passwordPermissionResource struct {
	client *tools.PassboltClient
}

type passwordPermissionModel struct {
	ID         types.String `tfsdk:"id"`
	ResourceID types.String `tfsdk:"resource_id"`
	GroupName  types.String `tfsdk:"group_name"`
	Username   types.String `tfsdk:"username"`
	Permission types.String `tfsdk:"permission"`
}

type passwordPermissionTarget struct {
	ARO  string
	ID   string
	Kind string
	Name string
}

func (r *passwordPermissionResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *PassboltClient, got: %T.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *passwordPermissionResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_password_permission"
}

func (r *passwordPermissionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("group_name"),
			path.MatchRoot("username"),
		),
	}
}

func (r *passwordPermissionResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Description: "Grants a Passbolt group or user permission to access a specific password/resource. " +
			"Use this resource when you need explicit `read`, `update`, or `owner` access instead of the " +
			"`passbolt_password.share_groups` convenience shortcut. To revoke access, remove this resource " +
			"from the configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				Description: "Internal resource ID in the format `resource_id:group:group_name` or " +
					"`resource_id:user:username`.",
			},
			"resource_id": schema.StringAttribute{
				Required:    true,
				Description: "The UUID of the Passbolt password/resource to share.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group_name": schema.StringAttribute{
				Optional: true,
				Description: "The name of the Passbolt group to grant access to. " +
					"Exactly one of `group_name` or `username` must be set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Optional: true,
				Description: "The exact username/email address of the Passbolt user to grant access to. " +
					"Exactly one of `group_name` or `username` must be set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permission": schema.StringAttribute{
				Required:    true,
				Description: "Level of access to grant. Must be one of `read`, `update`, or `owner`.",
				Validators: []validator.String{
					stringvalidator.OneOf("read", "update", "owner"),
				},
			},
		},
	}
}

func (r *passwordPermissionResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	importID, err := parsePasswordPermissionImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID format", err.Error())

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_id"), importID.ResourceID)...)
	switch importID.Kind {
	case passwordPermissionImportKindGroup:
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_name"), importID.Name)...)
	case passwordPermissionImportKindUser:
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("username"), importID.Name)...)
	}
}

func (r *passwordPermissionResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan passwordPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan, err := r.applyPasswordPermissionState(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Cannot share password", err.Error())

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *passwordPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state passwordPermissionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	target, err := resolvePasswordPermissionTarget(ctx, r.client, state, false)
	if err != nil {
		resp.Diagnostics.AddError("Cannot resolve permission target", err.Error())

		return
	}

	permission, err := readPasswordPermission(ctx, r.client, state.ResourceID.ValueString(), target)
	if err != nil {
		if errors.Is(err, errPasswordPermissionNotFound) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Cannot read password permission", err.Error())

		return
	}

	state.ID = types.StringValue(passwordPermissionID(state.ResourceID.ValueString(), target.Kind, target.Name))
	state.Permission = types.StringValue(permission)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *passwordPermissionResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan passwordPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan, err := r.applyPasswordPermissionState(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Cannot update password permission", err.Error())

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *passwordPermissionResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state passwordPermissionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	target, err := resolvePasswordPermissionTarget(ctx, r.client, state, false)
	if err != nil {
		resp.Diagnostics.AddError("Cannot resolve permission target", err.Error())

		return
	}

	if err := revokePasswordPermission(ctx, r.client, state.ResourceID.ValueString(), target); err != nil {
		resp.Diagnostics.AddError("Cannot revoke password permission", err.Error())
	}
}

func (r *passwordPermissionResource) applyPasswordPermissionState(
	ctx context.Context,
	plan passwordPermissionModel,
) (passwordPermissionModel, error) {
	target, err := resolvePasswordPermissionTarget(ctx, r.client, plan, true)
	if err != nil {
		return plan, fmt.Errorf("resolving permission target: %w", err)
	}

	err = applyPasswordPermission(ctx, r.client, plan.ResourceID.ValueString(), target, plan.Permission.ValueString())
	if err != nil {
		return plan, err
	}

	plan.ID = types.StringValue(passwordPermissionID(plan.ResourceID.ValueString(), target.Kind, target.Name))

	return plan, nil
}

func resolvePasswordPermissionTarget(
	ctx context.Context,
	client *tools.PassboltClient,
	model passwordPermissionModel,
	requireActiveUser bool,
) (passwordPermissionTarget, error) {
	if !model.GroupName.IsNull() && !model.GroupName.IsUnknown() && model.GroupName.ValueString() != "" {
		groupID, err := getgroupIDByName(ctx, client, model.GroupName.ValueString())
		if err != nil {
			return passwordPermissionTarget{}, err
		}

		return passwordPermissionTarget{
			ARO:  passwordPermissionAROGroup,
			ID:   groupID,
			Kind: passwordPermissionImportKindGroup,
			Name: model.GroupName.ValueString(),
		}, nil
	}

	if model.Username.IsNull() || model.Username.IsUnknown() || model.Username.ValueString() == "" {
		return passwordPermissionTarget{}, errors.New("exactly one of group_name or username must be set")
	}

	user, err := getUserByUsername(ctx, client, model.Username.ValueString(), !requireActiveUser)
	if err != nil {
		return passwordPermissionTarget{}, err
	}

	return passwordPermissionTarget{
		ARO:  passwordPermissionAROUser,
		ID:   user.ID,
		Kind: passwordPermissionImportKindUser,
		Name: model.Username.ValueString(),
	}, nil
}

func getUserByUsername(
	ctx context.Context,
	client *tools.PassboltClient,
	username string,
	includeInactive bool,
) (*api.User, error) {
	users, err := client.Client.GetUsers(ctx, &api.GetUsersOptions{
		FilterSearch: username,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	return userByUsername(users, username, includeInactive)
}

func applyPasswordPermission(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	target passwordPermissionTarget,
	permission string,
) error {
	permissionType, err := passwordPermissionStringToInt(permission)
	if err != nil {
		return err
	}

	current, err := readPasswordPermission(ctx, client, resourceID, target)
	if err == nil && current == permission {
		return nil
	}
	if err != nil && !errors.Is(err, errPasswordPermissionNotFound) {
		return err
	}

	return sharePasswordPermission(ctx, client, resourceID, target, permissionType)
}

func revokePasswordPermission(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	target passwordPermissionTarget,
) error {
	_, err := readPasswordPermission(ctx, client, resourceID, target)
	if errors.Is(err, errPasswordPermissionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	return sharePasswordPermission(ctx, client, resourceID, target, -1)
}

func sharePasswordPermission(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	target passwordPermissionTarget,
	permissionType int,
) error {
	users := []string(nil)
	groups := []string(nil)
	if target.ARO == passwordPermissionAROUser {
		users = []string{target.ID}
	} else {
		groups = []string{target.ID}
	}

	return helper.ShareResourceWithUsersAndGroups(ctx, client.Client, resourceID, users, groups, permissionType)
}

func readPasswordPermission(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	target passwordPermissionTarget,
) (string, error) {
	permissions, err := client.Client.GetResourcePermissions(ctx, resourceID)
	if err != nil {
		return "", passwordPermissionReadError(err)
	}

	for _, permission := range permissions {
		if permission.ARO != target.ARO || permission.AROForeignKey != target.ID {
			continue
		}

		value := passwordPermissionIntToString(permission.Type)
		if value == "" {
			return "", fmt.Errorf("unsupported remote permission type %d", permission.Type)
		}

		return value, nil
	}

	return "", errPasswordPermissionNotFound
}

func passwordPermissionReadError(err error) error {
	if isNotFoundError(err) {
		return errPasswordPermissionNotFound
	}

	return fmt.Errorf("getting resource permissions: %w", err)
}

func passwordPermissionStringToInt(permission string) (int, error) {
	switch permission {
	case "read":
		return 1, nil
	case "update":
		return 7, nil
	case "owner":
		return 15, nil
	}

	return 0, fmt.Errorf("%w: %q (must be read, update, owner)", errInvalidPermission, permission)
}

func passwordPermissionIntToString(permission int) string {
	switch permission {
	case 1:
		return "read"
	case 7:
		return "update"
	case 15:
		return "owner"
	default:
		return ""
	}
}

type passwordPermissionImportID struct {
	ResourceID string
	Kind       string
	Name       string
}

func parsePasswordPermissionImportID(value string) (passwordPermissionImportID, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 {
		return passwordPermissionImportID{}, fmt.Errorf(
			"expected format: <resource_id>:group:<group_name> or <resource_id>:user:<username>",
		)
	}

	if parts[0] == "" || parts[2] == "" {
		return passwordPermissionImportID{}, fmt.Errorf(
			"resource_id and target name must not be empty",
		)
	}

	if parts[1] != passwordPermissionImportKindGroup && parts[1] != passwordPermissionImportKindUser {
		return passwordPermissionImportID{}, fmt.Errorf(
			"target type must be %q or %q",
			passwordPermissionImportKindGroup,
			passwordPermissionImportKindUser,
		)
	}

	return passwordPermissionImportID{
		ResourceID: parts[0],
		Kind:       parts[1],
		Name:       parts[2],
	}, nil
}

func passwordPermissionID(resourceID string, kind string, name string) string {
	return fmt.Sprintf("%s:%s:%s", resourceID, kind, name)
}
