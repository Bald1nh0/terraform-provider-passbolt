package provider

import (
	"context"
	"fmt"
	"slices"

	"terraform-provider-passbolt/tools"

	"github.com/passbolt/go-passbolt/helper"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure interfaces
var (
	_ resource.Resource                = &groupResource{}
	_ resource.ResourceWithConfigure   = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
)

// NewGroupResource returns a Terraform resource for managing Passbolt groups.
func NewGroupResource() resource.Resource {
	return &groupResource{}
}

type groupResource struct {
	client *tools.PassboltClient
}

type groupModel struct {
	ID       types.String   `tfsdk:"id"`
	Name     types.String   `tfsdk:"name"`
	Managers []types.String `tfsdk:"managers"`
	Members  types.List     `tfsdk:"members"`
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Type",
			fmt.Sprintf("Expected *PassboltClient, got: %T",
				req.ProviderData))

		return
	}

	r.client = client
}

func (r *groupResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates and manages Passbolt groups. Groups can be assigned managers and used to share " +
			"resources like passwords or folders. Passbolt requires the authenticated API user to be a group " +
			"manager when changing memberships on an existing group. Group memberships also require existing " +
			"active Passbolt users; users created by passbolt_user may need to be activated before they can be " +
			"referenced from passbolt_group in a later apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Group ID (UUID).",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Group name.",
			},
			"managers": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "List of user IDs to assign as group managers. Users must already exist and be active in Passbolt.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.NoNullValues(),
					listvalidator.UniqueValues(),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
			"members": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "List of user IDs to assign as regular group members. " +
					"Users must already exist and be active in Passbolt.",
				Validators: []validator.List{
					listvalidator.NoNullValues(),
					listvalidator.UniqueValues(),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
		},
	}
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	members := listStringValues(ctx, plan.Members, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateGroupMembershipConfig(plan.Managers, members); err != nil {
		resp.Diagnostics.AddError("Invalid group membership", err.Error())

		return
	}

	ops := buildCreateGroupMembershipOps(plan.Managers, members)

	groupID, err := helper.CreateGroup(ctx, r.client.Client, plan.Name.ValueString(), ops)
	if err != nil {
		resp.Diagnostics.AddError("Error creating group", err.Error())

		return
	}

	plan.ID = types.StringValue(groupID)
	plan.Members = listStringValue(members)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name, memberships, err := helper.GetGroup(ctx, r.client.Client, state.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)

		return
	}

	state.Name = types.StringValue(name)
	managerIDs, memberIDs := splitGroupMemberships(memberships, true)
	state.Managers = managerIDs
	state.Members = listStringValue(memberIDs)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupModel
	var state groupModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planMembers := listStringValues(ctx, plan.Members, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateGroupMembershipConfig(plan.Managers, planMembers); err != nil {
		resp.Diagnostics.AddError("Invalid group membership", err.Error())

		return
	}

	_, memberships, err := helper.GetGroup(ctx, r.client.Client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading group memberships", err.Error())

		return
	}
	stateManagers, stateMembers := splitGroupMemberships(memberships, true)

	ops := buildGroupMembershipOps(plan.Managers, planMembers, stateManagers, stateMembers)

	err = updateGroup(ctx, r.client.Client, state.ID.ValueString(), plan.Name.ValueString(), ops)
	if err != nil {
		resp.Diagnostics.AddError("Error updating group", err.Error())

		return
	}

	plan.ID = state.ID
	plan.Members = listStringValue(planMembers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func listStringValues(ctx context.Context, list types.List, diags *diag.Diagnostics) []types.String {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}

	var values []types.String
	diags.Append(list.ElementsAs(ctx, &values, false)...)

	return values
}

func listStringValue(values []types.String) types.List {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, value)
	}

	return types.ListValueMust(types.StringType, elements)
}

func splitGroupMemberships(
	memberships []helper.GroupMembership,
	includeMembers bool,
) ([]types.String, []types.String) {
	managerIDs := make([]types.String, 0, len(memberships))
	memberIDs := make([]types.String, 0, len(memberships))

	for _, membership := range memberships {
		if membership.IsGroupManager {
			managerIDs = append(managerIDs, types.StringValue(membership.UserID))
		} else if includeMembers {
			memberIDs = append(memberIDs, types.StringValue(membership.UserID))
		}
	}

	return managerIDs, memberIDs
}

func validateGroupMembershipConfig(managers, members []types.String) error {
	if len(managers) == 0 {
		return fmt.Errorf("at least one group manager is required")
	}

	overlap := overlappingGroupUsers(managers, members)
	if len(overlap) > 0 {
		return fmt.Errorf("users cannot be both managers and members: %v", overlap)
	}

	return nil
}

func overlappingGroupUsers(managers, members []types.String) []string {
	managerSet := groupUserSet(managers)
	overlap := make([]string, 0, len(members))

	for _, uid := range members {
		userID := uid.ValueString()
		if managerSet[userID] {
			overlap = append(overlap, userID)
		}
	}

	slices.Sort(overlap)

	return overlap
}

func buildCreateGroupMembershipOps(managers, members []types.String) []helper.GroupMembershipOperation {
	ops := make([]helper.GroupMembershipOperation, 0, len(managers)+len(members))

	for _, uid := range managers {
		ops = append(ops, helper.GroupMembershipOperation{
			UserID:         uid.ValueString(),
			IsGroupManager: true,
		})
	}

	for _, uid := range members {
		ops = append(ops, helper.GroupMembershipOperation{
			UserID:         uid.ValueString(),
			IsGroupManager: false,
		})
	}

	return ops
}

func buildGroupMembershipOps(
	planManagers,
	planMembers,
	stateManagers,
	stateMembers []types.String,
) []helper.GroupMembershipOperation {
	desired := groupUserRoleMap(planManagers, planMembers)
	current := groupUserRoleMap(stateManagers, stateMembers)
	userIDs := groupUserIDs(desired, current)

	ops := make([]helper.GroupMembershipOperation, 0, len(userIDs))
	ops = appendMembershipRoleChanges(ops, userIDs, desired, current, true)
	ops = appendNewGroupMembers(ops, userIDs, desired, current)
	ops = appendMembershipRoleChanges(ops, userIDs, desired, current, false)
	ops = appendRemovedGroupUsers(ops, userIDs, desired, current)

	return ops
}

func appendMembershipRoleChanges(
	ops []helper.GroupMembershipOperation,
	userIDs []string,
	desired,
	current map[string]bool,
	isGroupManager bool,
) []helper.GroupMembershipOperation {
	for _, uid := range userIDs {
		desiredRole, desiredExists := desired[uid]
		currentRole := current[uid]
		if !desiredExists || desiredRole != isGroupManager || currentRole == desiredRole {
			continue
		}
		ops = append(ops, helper.GroupMembershipOperation{
			UserID:         uid,
			IsGroupManager: desiredRole,
		})
	}

	return ops
}

func appendNewGroupMembers(
	ops []helper.GroupMembershipOperation,
	userIDs []string,
	desired,
	current map[string]bool,
) []helper.GroupMembershipOperation {
	for _, uid := range userIDs {
		desiredRole, desiredExists := desired[uid]
		if !desiredExists || desiredRole {
			continue
		}
		if _, currentExists := current[uid]; currentExists {
			continue
		}
		ops = append(ops, helper.GroupMembershipOperation{
			UserID:         uid,
			IsGroupManager: false,
		})
	}

	return ops
}

func appendRemovedGroupUsers(
	ops []helper.GroupMembershipOperation,
	userIDs []string,
	desired,
	current map[string]bool,
) []helper.GroupMembershipOperation {
	for _, uid := range userIDs {
		if _, currentExists := current[uid]; !currentExists {
			continue
		}
		if _, desiredExists := desired[uid]; desiredExists {
			continue
		}
		ops = append(ops, helper.GroupMembershipOperation{
			UserID: uid,
			Delete: true,
		})
	}

	return ops
}

func groupUserRoleMap(managers, members []types.String) map[string]bool {
	roles := make(map[string]bool, len(managers)+len(members))

	for _, uid := range members {
		roles[uid.ValueString()] = false
	}

	for _, uid := range managers {
		roles[uid.ValueString()] = true
	}

	return roles
}

func groupUserSet(users []types.String) map[string]bool {
	result := make(map[string]bool, len(users))
	for _, uid := range users {
		result[uid.ValueString()] = true
	}

	return result
}

func groupUserIDs(roleMaps ...map[string]bool) []string {
	seen := make(map[string]bool)
	for _, roleMap := range roleMaps {
		for uid := range roleMap {
			seen[uid] = true
		}
	}

	userIDs := make([]string, 0, len(seen))
	for uid := range seen {
		userIDs = append(userIDs, uid)
	}
	slices.Sort(userIDs)

	return userIDs
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := helper.DeleteGroup(ctx, r.client.Client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting group", err.Error())
	}
}
