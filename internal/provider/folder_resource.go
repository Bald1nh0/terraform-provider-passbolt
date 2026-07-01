package provider

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &folderResource{}
	_ resource.ResourceWithConfigure   = &folderResource{}
	_ resource.ResourceWithImportState = &folderResource{}
)

// NewFolderResource returns interface a new instance of folderResource that implements the resource.Resource interface.
func NewFolderResource() resource.Resource {
	return &folderResource{}
}

// folderResource is the resource implementation.
type folderResource struct {
	client *tools.PassboltClient
}

// created, modified, created_by, modified_by, and folder_parent_id
type foldersModelCreate struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	FolderParent       types.String `tfsdk:"folder_parent"`
	FolderParentID     types.String `tfsdk:"folder_parent_id"`
	Personal           types.Bool   `tfsdk:"personal"`
	MetadataType       types.String `tfsdk:"metadata_type"`
	MetadataTypeActual types.String `tfsdk:"metadata_type_actual"`
}

// Configure adds the provider configured client to the resource.
func (r *folderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf(
				"Invalid provider data type: %T",
				req.ProviderData,
			),
		)

		return
	}

	r.client = client
}

func (r *folderResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// Metadata returns the resource type name.
func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Schema defines the schema for the resource.
func (r *folderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates a folder in Passbolt. Folders are used to organize secrets and can be shared with" +
			" groups using the `passbolt_folder_permission` resource.\n\n" +
			"Folders can optionally have a parent folder (nesting is supported).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the Passbolt folder.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name for the folder. Must be unique among sibling folders.",
			},
			"personal": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Passbolt currently treats the folder as personal, meaning it is not shared.",
			},

			"folder_parent": schema.StringAttribute{
				Optional: true,
				Description: "Reference to the parent folder. Accepts a unique folder name, " +
					"a folder UUID, or an absolute path such as `/application_A/prod`. " +
					"If omitted, the folder will be created at the top level.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"folder_parent_id": schema.StringAttribute{
				Computed:    true,
				Description: "Resolved UUID of the parent folder.",
			},
			"metadata_type": schema.StringAttribute{
				Optional: true,
				Description: "Optional metadata format for this folder. Use `v5` to create or migrate the folder " +
					"to encrypted metadata, `v4` to force legacy cleartext metadata on create, or leave unset to " +
					"use the Passbolt server default without migrating existing folders.",
				Validators: []validator.String{
					stringvalidator.OneOf(metadataTypeV4, metadataTypeV5, metadataTypeServerDefault),
				},
			},
			"metadata_type_actual": schema.StringAttribute{
				Computed:      true,
				Description:   "Actual remote metadata format for this folder: `v4` or `v5`.",
				PlanModifiers: metadataTypeActualPlanModifiers(),
			},
		},
	}
}

// Create a new resource.
func (r *folderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Create folder resource")

	var plan foldersModelCreate
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normalize "" to null to prevent drift
	if plan.FolderParent.ValueString() == "" {
		plan.FolderParent = types.StringNull()
	}

	folderID, diags := resolveFolderReference(ctx, r.client, plan.FolderParent)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cFolder, metadataTypeActual, errCreate := createPassboltFolder(
		ctx,
		r.client,
		folderID,
		plan.Name.ValueString(),
		desiredMetadataType(plan.MetadataType),
	)
	if errCreate != nil {
		resp.Diagnostics.AddError("Error creating folder", "Could not create folder: "+errCreate.Error())

		return
	}

	plan.ID = types.StringValue(cFolder.ID)
	plan.FolderParentID = pickOptional(folderID)
	plan.Personal = types.BoolValue(cFolder.Personal)
	plan.MetadataTypeActual = types.StringValue(metadataTypeActual)

	tflog.Info(ctx, "Created folder", map[string]any{
		"id":       cFolder.ID,
		"personal": cFolder.Personal,
	})

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *folderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Info(ctx, "Read folder resource")
	var state foldersModelCreate
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawFolder, currentFolder, err := getPassboltFolder(ctx, r.client, state.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Cannot get folder", err.Error())

		return
	}

	folders, err := getPassboltFolders(ctx, r.client, nil)
	if err != nil {
		resp.Diagnostics.AddError("Cannot get folders", err.Error())

		return
	}

	for _, f := range folders {
		if f.ID == currentFolder.ID {
			state.Name = types.StringValue(f.Name)
			state.ID = types.StringValue(f.ID)
			state.FolderParent = reconcileFolderParentReference(state.FolderParent, f.FolderParentID, folders)
			state.FolderParentID = pickOptional(f.FolderParentID)
			state.Personal = types.BoolValue(f.Personal)
			state.MetadataTypeActual = types.StringValue(actualMetadataTypeFromEncryptedMetadata(rawFolder.Metadata))

			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *folderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Update folder resource: starting")

	var plan foldersModelCreate
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Update: failed to get plan")

		return
	}

	var state foldersModelCreate
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Update: failed to get state")

		return
	}

	tflog.Debug(ctx, "Update folder", map[string]any{
		"id":   state.ID.ValueString(),
		"name": plan.Name.ValueString(),
	})

	plan.FolderParent = normalizeFolderParent(plan.FolderParent)
	desiredParentID, diags := resolveFolderReference(ctx, r.client, plan.FolderParent)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.moveFolderIfNeeded(ctx, state, desiredParentID, resp) {
		return
	}

	metadataTypeActual, err := updatePassboltFolder(
		ctx,
		r.client,
		state.ID.ValueString(),
		plan.Name.ValueString(),
		desiredMetadataType(plan.MetadataType),
	)
	if err != nil {
		tflog.Error(ctx, "Update: API update failed", map[string]any{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError("Cannot update folder", err.Error())

		return
	}

	plan = finalizeFolderPlan(plan, state, desiredParentID)
	plan.MetadataTypeActual = types.StringValue(metadataTypeActual)

	tflog.Info(ctx, "Update folder resource: applying state", map[string]any{
		"id":       plan.ID.ValueString(),
		"personal": plan.Personal.ValueBool(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *folderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state foldersModelCreate
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Client.DeleteFolder(ctx, state.ID.ValueString())
	if err != nil {
		if !isNotFoundError(err) {
			resp.Diagnostics.AddError("Error deleting folder", err.Error())
		}
	}
}

func normalizeFolderParent(parent types.String) types.String {
	if parent.IsUnknown() || parent.IsNull() || parent.ValueString() != "" {
		return parent
	}

	return types.StringNull()
}

func (r *folderResource) moveFolderIfNeeded(
	ctx context.Context,
	state foldersModelCreate,
	desiredParentID string,
	resp *resource.UpdateResponse,
) bool {
	if desiredParentID == state.FolderParentID.ValueString() {
		return true
	}

	if err := r.client.Client.MoveFolder(ctx, state.ID.ValueString(), desiredParentID); err != nil {
		tflog.Error(ctx, "Update: API move failed", map[string]any{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError("Cannot move folder", err.Error())

		return false
	}

	return true
}

func finalizeFolderPlan(plan, state foldersModelCreate, desiredParentID string) foldersModelCreate {
	plan.ID = state.ID
	plan.FolderParentID = pickOptional(desiredParentID)
	plan.FolderParent = normalizeFolderParent(plan.FolderParent)

	if plan.Personal.IsUnknown() {
		plan.Personal = state.Personal
	}

	return plan
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()

	return strings.Contains(message, "The folder does not exist.") ||
		strings.Contains(message, "The resource does not exist.")
}
