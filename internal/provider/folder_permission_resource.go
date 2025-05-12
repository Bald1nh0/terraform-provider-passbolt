// Package provider implements Terraform resources for Passbolt.
package provider

import (
	"context"
	"errors"
	"fmt"

	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

// Ensure implementation
var (
	_ resource.Resource              = &folderPermissionResource{}
	_ resource.ResourceWithConfigure = &folderPermissionResource{}
)

// NewFolderPermissionResource returns a Terraform resource for managing Passbolt folder permissions.
func NewFolderPermissionResource() resource.Resource {
	return &folderPermissionResource{}
}

type folderPermissionResource struct {
	client *tools.PassboltClient
}

type folderPermissionModel struct {
	ID         types.String `tfsdk:"id"`
	FolderID   types.String `tfsdk:"folder_id"`
	GroupName  types.String `tfsdk:"group_name"`
	Permission types.String `tfsdk:"permission"` // "read", "update", "delete", "owner"
}

// Provide client
func (r *folderPermissionResource) Configure(
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
			fmt.Sprintf("Expected *PassboltClient, got: %T.", req.ProviderData))

		return
	}
	r.client = client
}

// Metadata for resource
func (r *folderPermissionResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_folder_permission"
}

// Schema
func (r *folderPermissionResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Internal ID is always folder_id:group_name",
			},
			"folder_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of Passbolt folder to be shared",
			},
			"group_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Passbolt group to share with",
			},
			"permission": schema.StringAttribute{
				Required:    true,
				Description: "Permission type: read, update, delete, or owner",
			},
		},
	}
}

// Create
func (r *folderPermissionResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan folderPermissionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, err := getgroupIDByName(ctx, r.client, plan.GroupName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Group Not Found", err.Error())

		return
	}

	permInt, err := permissionStringToInt(plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Permission Mapping Error", err.Error())

		return
	}

	err = helper.ShareFolderWithUsersAndGroups(
		ctx,
		r.client.Client,
		plan.FolderID.ValueString(),
		nil,
		[]string{groupID},
		permInt,
	)
	if err != nil {
		resp.Diagnostics.AddError("Cannot share folder (helper)", err.Error())

		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.FolderID.ValueString(), plan.GroupName.ValueString()))
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read - return state
func (r *folderPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state folderPermissionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, err := getgroupIDByName(ctx, r.client, state.GroupName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving group ID", err.Error())

		return
	}

	folders, err := r.client.Client.GetFolders(ctx, &api.GetFoldersOptions{
		ContainPermissions: true,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error fetching folders", err.Error())

		return
	}

	for _, folder := range folders {
		if folder.ID != state.FolderID.ValueString() {
			continue
		}

		var latestPermType *int

		for _, perm := range folder.Permissions {
			if perm.ARO == "Group" && perm.AROForeignKey == groupID {
				latestPermType = &perm.Type

				break
			}
		}

		if latestPermType != nil {
			state.Permission = types.StringValue(permissionIntToString(*latestPermType))
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		} else {
			resp.State.RemoveResource(ctx)
		}

		return
	}

	resp.State.RemoveResource(ctx)
}

func permissionIntToString(perm int) string {
	switch perm {
	case 1:
		return "read"
	case 7:
		return "update"
	case 15:
		return "owner"
	case -1:
		return "delete"
	default:
		return "unknown"
	}
}

func (r *folderPermissionResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan folderPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	groupID, err := getgroupIDByName(ctx, r.client, plan.GroupName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Group Not Found", err.Error())

		return
	}

	permInt, err := permissionStringToInt(plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Permission Mapping Error", err.Error())

		return
	}

	err = helper.ShareFolderWithUsersAndGroups(
		ctx,
		r.client.Client,
		plan.FolderID.ValueString(),
		nil,
		[]string{groupID},
		permInt,
	)
	if err != nil {
		resp.Diagnostics.AddError("Cannot update folder permission", err.Error())

		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.FolderID.ValueString(), plan.GroupName.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete (revoke sharing)
func (r *folderPermissionResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state folderPermissionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, err := getgroupIDByName(ctx, r.client, state.GroupName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Group Not Found In Delete", err.Error())

		return
	}
	// ShareFolderWithUsersAndGroups with perm=-1 – remove all permissions
	err = helper.ShareFolderWithUsersAndGroups(
		ctx,
		r.client.Client,
		state.FolderID.ValueString(),
		nil,
		[]string{groupID},
		-1,
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to unshare folder", err.Error())

		return
	}
}

// ----------------- Helpers ------------------

var (
	errGroupNotFound     = errors.New("group not found")
	errInvalidPermission = errors.New("invalid permission")
)

func getgroupIDByName(ctx context.Context, client *tools.PassboltClient, groupName string) (string, error) {
	groups, err := client.Client.GetGroups(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get groups: %w", err)
	}
	for _, group := range groups {
		if group.Name == groupName {
			return group.ID, nil
		}
	}

	return "", fmt.Errorf("%w: %q", errGroupNotFound, groupName)
}

func permissionStringToInt(perm string) (int, error) {
	switch perm {
	case "read":
		return 1, nil
	case "update":
		return 7, nil
	case "owner":
		return 15, nil
	case "delete":
		return -1, nil
	}

	return 0, fmt.Errorf("%w: %q (must be read, update, owner, delete)", errInvalidPermission, perm)
}
