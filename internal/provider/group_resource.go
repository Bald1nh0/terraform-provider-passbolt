package provider

import (
	"context"
	"fmt"

	"terraform-provider-passbolt/tools"

	"github.com/passbolt/go-passbolt/helper"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
			"resources like passwords or folders.",
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
				Description: "List of user IDs to assign as group managers.",
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

	ops := make([]helper.GroupMembershipOperation, 0, len(plan.Managers))
	for _, uid := range plan.Managers {
		ops = append(ops, helper.GroupMembershipOperation{
			UserID:         uid.ValueString(),
			IsGroupManager: true,
		})
	}

	groupID, err := helper.CreateGroup(ctx, r.client.Client, plan.Name.ValueString(), ops)
	if err != nil {
		resp.Diagnostics.AddError("Error creating group", err.Error())

		return
	}

	plan.ID = types.StringValue(groupID)
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
	var managerIDs []types.String
	for _, m := range memberships {
		if m.IsGroupManager {
			managerIDs = append(managerIDs, types.StringValue(m.UserID))
		}
	}
	state.Managers = managerIDs

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

	added, removed := detectManagerChanges(plan.Managers, state.Managers)

	ops := buildGroupMembershipOps(added, removed)

	err := helper.UpdateGroup(ctx, r.client.Client, state.ID.ValueString(), plan.Name.ValueString(), ops)
	if err != nil {
		resp.Diagnostics.AddError("Error updating group", err.Error())

		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func detectManagerChanges(planManagers, stateManagers []types.String) (added, removed map[string]bool) {
	newSet := make(map[string]bool)
	removed = make(map[string]bool)
	added = make(map[string]bool)

	for _, uid := range planManagers {
		newSet[uid.ValueString()] = true
	}

	for _, old := range stateManagers {
		oldID := old.ValueString()
		if !newSet[oldID] {
			removed[oldID] = true
		}
	}

	for uid := range newSet {
		if !contains(stateManagers, uid) {
			added[uid] = true
		}
	}

	return
}

func buildGroupMembershipOps(added, removed map[string]bool) []helper.GroupMembershipOperation {
	ops := make([]helper.GroupMembershipOperation, 0, len(added)+len(removed))

	for uid := range added {
		ops = append(ops, helper.GroupMembershipOperation{
			UserID:         uid,
			IsGroupManager: true,
		})
	}
	for uid := range removed {
		ops = append(ops, helper.GroupMembershipOperation{
			UserID: uid,
			Delete: true,
		})
	}

	return ops
}

func contains(users []types.String, uid string) bool {
	for _, u := range users {
		if u.ValueString() == uid {
			return true
		}
	}

	return false
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
