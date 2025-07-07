package provider

import (
	"context"
	"fmt"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

var (
	_ resource.Resource                = &passwordResource{}
	_ resource.ResourceWithConfigure   = &passwordResource{}
	_ resource.ResourceWithImportState = &passwordResource{}
)

// NewPasswordResource returns a new instance of passwordResource as a Terraform resource.
func NewPasswordResource() resource.Resource {
	return &passwordResource{}
}

type passwordResource struct {
	client *tools.PassboltClient
}

type passwordModel struct {
	ID           types.String   `tfsdk:"id"`
	Name         types.String   `tfsdk:"name"`
	Description  types.String   `tfsdk:"description"`
	Username     types.String   `tfsdk:"username"`
	URI          types.String   `tfsdk:"uri"`
	ShareGroup   types.String   `tfsdk:"share_group"`
	ShareGroups  []types.String `tfsdk:"share_groups"`
	FolderParent types.String   `tfsdk:"folder_parent"`
	Password     types.String   `tfsdk:"password"`
}

func (r *passwordResource) Configure(
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
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *PassboltClient, got: %T", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *passwordResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *passwordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password"
}

func (r *passwordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a secret/password entry in Passbolt. Supports optional folder placement and group sharing.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the Passbolt password/secret resource. Used for import and internal tracking.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name for identifying the password/secret in Passbolt.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Free-form description for this password/secret.",
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username or login for the password/secret entry.",
			},
			"uri": schema.StringAttribute{
				Required:    true,
				Description: "The URI or URL where the secret is used (e.g., https://service.example.com).",
			},
			"share_group": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the Passbolt group to share this secret with. Leave unset to keep private.",
			},
			"share_groups": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "List of Passbolt group names to share this secret with. Supports multiple group " +
					"shares. Takes precedence over `share_group`.",
			},
			"folder_parent": schema.StringAttribute{
				Optional:    true,
				Description: "Name or UUID of an existing folder to place the secret in. Leave unset to place at top level.",
			},
			"password": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "The actual secret or password value. Marked sensitive — " +
					"will not appear in CLI output or state diffs.",
			},
		},
	}
}

func (r *passwordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan passwordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	folderID, diag := resolveFolderID(ctx, r.client, plan.FolderParent)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceID, err := helper.CreateResource(
		ctx,
		r.client.Client,
		folderID,
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		plan.Password.ValueString(),
		plan.Description.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Cannot create resource", err.Error())

		return
	}

	plan.ID = types.StringValue(resourceID)

	if len(plan.ShareGroups) > 0 {
		shareResourceWithGroups(ctx, r.client, plan.ShareGroups, resourceID, &resp.Diagnostics)
	} else if !plan.ShareGroup.IsUnknown() && !plan.ShareGroup.IsNull() && plan.ShareGroup.ValueString() != "" {
		shareResourceWithGroups(ctx, r.client, []types.String{plan.ShareGroup}, resourceID, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// resolveFolderId can now match both name and UUID
func resolveFolderID(
	ctx context.Context,
	client *tools.PassboltClient,
	folder types.String) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if folder.IsUnknown() || folder.IsNull() {
		return "", diags
	}

	value := folder.ValueString()
	folders, err := client.Client.GetFolders(ctx, nil)
	if err != nil {
		diags.AddError("Cannot get folders", err.Error())

		return "", diags
	}

	for _, f := range folders {
		if f.ID == value || f.Name == value {
			return f.ID, diags
		}
	}

	diags.AddError("Folder not found", fmt.Sprintf("Folder with name or ID '%s' not found", value))

	return "", diags
}

func shareResourceWithGroups(
	ctx context.Context,
	client *tools.PassboltClient,
	groupNames []types.String,
	resourceID string,
	diags *diag.Diagnostics,
) {
	if len(groupNames) == 0 {
		return
	}

	groupsByName, groupErr := buildGroupNameMap(ctx, client, diags)
	if groupErr != nil {
		return
	}

	existingPerms, permErr := getExistingGroupPermissions(ctx, client, resourceID, diags)
	if permErr != nil {
		return
	}

	changes := make([]helper.ShareOperation, 0, len(groupNames))
	for _, name := range groupNames {
		groupName := name.ValueString()
		group, ok := groupsByName[groupName]
		if !ok {
			diags.AddError("Group not found", fmt.Sprintf("Group with name '%s' not found", groupName))

			continue
		}
		if existingPerms[group.ID] == 7 {
			continue
		}
		changes = append(changes, helper.ShareOperation{
			Type:  7,
			ARO:   "Group",
			AROID: group.ID,
		})
	}

	if len(changes) > 0 {
		if err := helper.ShareResource(ctx, client.Client, resourceID, changes); err != nil {
			diags.AddError("Cannot share resource", err.Error())
		}
	}
}

func buildGroupNameMap(
	ctx context.Context,
	client *tools.PassboltClient,
	diags *diag.Diagnostics,
) (map[string]api.Group, error) {
	groups, err := client.Client.GetGroups(ctx, nil)
	if err != nil {
		diags.AddError("Cannot get groups", err.Error())

		return nil, err
	}
	result := make(map[string]api.Group)
	for _, g := range groups {
		result[g.Name] = g
	}

	return result, nil
}

func getExistingGroupPermissions(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	diags *diag.Diagnostics,
) (map[string]int, error) {
	perms, err := client.Client.GetResourcePermissions(ctx, resourceID)
	if err != nil {
		diags.AddError("Cannot get existing permissions", err.Error())

		return nil, err
	}
	result := make(map[string]int)
	for _, p := range perms {
		if p.ARO == "Group" {
			result[p.AROForeignKey] = p.Type
		}
	}

	return result, nil
}

func buildPasswordState(
	ctx context.Context,
	client *tools.PassboltClient,
	id string,
	existing passwordModel,
) (passwordModel, diag.Diagnostics) {
	var state passwordModel
	var diags diag.Diagnostics

	folderID, name, username, uri, password, description, err := helper.GetResource(ctx, client.Client, id)
	if err != nil {
		diags.AddError("Cannot read resource", err.Error())

		return state, diags
	}

	state.ID = types.StringValue(id)
	state.Name = types.StringValue(name)
	state.Username = types.StringValue(username)
	state.URI = types.StringValue(uri)

	state.Password = pickPassword(password, existing.Password)
	state.Description = pickOptional(description)
	state.FolderParent = pickOptional(folderID)

	if description == "" {
		state.Description = types.StringNull()
	} else {
		state.Description = types.StringValue(description)
	}

	if folderID == "" {
		state.FolderParent = types.StringNull()
	} else {
		state.FolderParent = types.StringValue(folderID)
	}

	if existing.ShareGroup.IsUnknown() || existing.ShareGroup.IsNull() || existing.ShareGroup.ValueString() == "" {
		state.ShareGroup = types.StringNull()
	} else {
		state.ShareGroup = existing.ShareGroup
	}
	if len(existing.ShareGroups) > 0 {
		state.ShareGroups = existing.ShareGroups
	} else {
		state.ShareGroups = nil
	}

	return state, diags
}

func pickPassword(actual string, existing types.String) types.String {
	if actual != "" {
		return types.StringValue(actual)
	}
	if existing.IsUnknown() || existing.IsNull() {
		return types.StringNull()
	}

	return existing
}

func pickOptional(value string) types.String {
	if value == "" {
		return types.StringNull()
	}

	return types.StringValue(value)
}

// Read retrieves the current state of the resource from Passbolt.
func (r *passwordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state passwordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := buildPasswordState(ctx, r.client, state.ID.ValueString(), state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *passwordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan passwordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state passwordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := updateResourceFields(ctx, r, plan, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)

		return
	}

	plan.ID = state.ID // preserve original ID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func updateResourceFields(
	ctx context.Context,
	r *passwordResource,
	plan passwordModel,
	state passwordModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	err := helper.UpdateResource(
		ctx,
		r.client.Client,
		state.ID.ValueString(),
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		plan.Password.ValueString(),
		plan.Description.ValueString(),
	)
	if err != nil {
		diags.AddError("Error updating resource", err.Error())

		return diags
	}

	// Handle folder move
	if !plan.FolderParent.IsUnknown() && plan.FolderParent.ValueString() != state.FolderParent.ValueString() {
		newFolderID, folderDiags := resolveFolderID(ctx, r.client, plan.FolderParent)
		diags.Append(folderDiags...)
		if !diags.HasError() {
			if moveErr := r.client.Client.MoveResource(ctx, state.ID.ValueString(), newFolderID); moveErr != nil {
				diags.AddError("Error moving resource to folder", moveErr.Error())
			}
		}
	}

	// Handle re-sharing
	if len(plan.ShareGroups) > 0 {
		shareResourceWithGroups(ctx, r.client, plan.ShareGroups, state.ID.ValueString(), &diags)
	} else if !plan.ShareGroup.IsUnknown() && !plan.ShareGroup.IsNull() && plan.ShareGroup.ValueString() != "" {
		shareResourceWithGroups(ctx, r.client, []types.String{plan.ShareGroup}, state.ID.ValueString(), &diags)
	}

	return diags
}

func (r *passwordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state passwordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Client.DeleteResource(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting password", err.Error())
	}
}
