package provider

import (
	"context"
	"fmt"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/passbolt/go-passbolt/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &folderResource{}
	_ resource.ResourceWithConfigure = &folderResource{}
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
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	FolderParent types.String `tfsdk:"folder_parent"`
	Personal     types.Bool   `tfsdk:"personal"`
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

// Metadata returns the resource type name.
func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Schema defines the schema for the resource.
func (r *folderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
				Description: "Whether the folder is a personal folder. Always false for Terraform-created folders.",
			},

			"folder_parent": schema.StringAttribute{
				Optional: true,
				Description: "Name of the parent folder to create this one under. " +
					"If omitted, the folder will be created at the top level. " +
					"If set, this must be a valid folder name (not empty).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
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

	folders, err := r.client.Client.GetFolders(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Cannot get folders", "")

		return
	}

	var folderID string
	if !plan.FolderParent.IsUnknown() && !plan.FolderParent.IsNull() {
		for _, folder := range folders {
			if folder.Name == plan.FolderParent.ValueString() {
				folderID = folder.ID
			}
		}
	}

	cFolder, errCreate := r.client.Client.CreateFolder(ctx, api.Folder{
		FolderParentID: folderID,
		Name:           plan.Name.ValueString(),
		Personal:       false,
	})
	if errCreate != nil {
		resp.Diagnostics.AddError("Error creating folder", "Could not create folder: "+errCreate.Error())

		return
	}

	plan.ID = types.StringValue(cFolder.ID)
	plan.Personal = types.BoolValue(cFolder.Personal)

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

	folders, err := r.client.Client.GetFolders(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Cannot get folders", err.Error())

		return
	}

	for _, f := range folders {
		if f.ID == state.ID.ValueString() {
			state.Name = types.StringValue(f.Name)
			state.ID = types.StringValue(f.ID)

			if f.FolderParentID == "" {
				state.FolderParent = types.StringNull()
			} else {
				state.FolderParent = types.StringValue(f.FolderParentID)
			}

			state.Personal = types.BoolValue(f.Personal)

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

	_, err := r.client.Client.UpdateFolder(ctx, state.ID.ValueString(), api.Folder{
		Name: plan.Name.ValueString(),
	})
	if err != nil {
		tflog.Error(ctx, "Update: API update failed", map[string]any{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError("Cannot update folder", err.Error())

		return
	}

	// Preserve required fields to avoid drift
	plan.ID = state.ID

	if plan.FolderParent.IsUnknown() || plan.FolderParent.ValueString() == "" {
		plan.FolderParent = types.StringNull()
	}

	if plan.Personal.IsUnknown() {
		plan.Personal = state.Personal
	}

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

func isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "The folder does not exist." || err.Error() == "The resource does not exist.")
}
