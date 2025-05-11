package provider

import (
	"context"
	"fmt"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/helper"
)

var (
	_ resource.Resource              = &passwordResource{}
	_ resource.ResourceWithConfigure = &passwordResource{}
)

// NewPasswordResource returns a new instance of passwordResource as a Terraform resource.
func NewPasswordResource() resource.Resource {
	return &passwordResource{}
}

type passwordResource struct {
	client *tools.PassboltClient
}

type passwordModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Username     types.String `tfsdk:"username"`
	URI          types.String `tfsdk:"uri"`
	ShareGroup   types.String `tfsdk:"share_group"`
	FolderParent types.String `tfsdk:"folder_parent"`
	Password     types.String `tfsdk:"password"`
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

func (r *passwordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password"
}

func (r *passwordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the Passbolt password/secret resource.",
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
				Description: "A URI or reference where this password is used (e.g., https://service.example.com).",
			},
			"share_group": schema.StringAttribute{
				Optional:    true,
				Description: "Name of Passbolt group to share this secret with. Optional: omit to leave unshared.",
			},
			"folder_parent": schema.StringAttribute{
				Optional:    true,
				Description: "Name of an existing folder in Passbolt to place this secret in. Optional.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "The password or secret value. (Sensitive, will not be displayed in Terraform output.)",
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

	if !plan.ShareGroup.IsUnknown() && !plan.FolderParent.IsNull() {
		shareResourceWithGroup(ctx, r.client, plan.ShareGroup.ValueString(), resourceID, &resp.Diagnostics)
	}

	plan.ID = types.StringValue(resourceID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func resolveFolderID(
	ctx context.Context,
	client *tools.PassboltClient,
	folder types.String,
) (
	string,
	diag.Diagnostics,
) {
	var diags diag.Diagnostics

	if folder.IsUnknown() || folder.IsNull() {
		return "", diags
	}

	available, err := client.Client.GetFolders(ctx, nil)
	if err != nil {
		diags.AddError("Cannot get folders", err.Error())

		return "", diags
	}

	for _, f := range available {
		if f.Name == folder.ValueString() {
			return f.ID, diags
		}
	}

	diags.AddError("Folder not found", fmt.Sprintf("Folder with name '%s' not found", folder.ValueString()))

	return "", diags
}

func shareResourceWithGroup(
	ctx context.Context,
	client *tools.PassboltClient,
	groupName, resourceID string,
	diags *diag.Diagnostics,
) {
	groups, err := client.Client.GetGroups(ctx, nil)
	if err != nil {
		diags.AddError("Cannot get groups", err.Error())

		return
	}

	for _, group := range groups {
		if group.Name == groupName {
			shares := []helper.ShareOperation{
				{
					Type:  7,
					ARO:   "Group",
					AROID: group.ID,
				},
			}
			if err := helper.ShareResource(ctx, client.Client, resourceID, shares); err != nil {
				diags.AddError("Cannot share resource", err.Error())
			}

			return
		}
	}
	diags.AddError("Group not found", fmt.Sprintf("Group with name '%s' not found", groupName))
}

func (r *passwordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state passwordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	folderID, name, username, uri, password, description, err := helper.GetResource(
		ctx, r.client.Client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Cannot read resource", err.Error())

		return
	}

	state.Name = types.StringValue(name)
	state.Username = types.StringValue(username)
	state.URI = types.StringValue(uri)
	state.Password = types.StringValue(password)
	state.Description = types.StringValue(description)
	state.FolderParent = types.StringValue(folderID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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

	if err := r.client.Client.DeleteResource(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting resource", err.Error())

		return
	}

	r.Create(ctx, resource.CreateRequest{Plan: req.Plan}, &resource.CreateResponse{
		Diagnostics: resp.Diagnostics,
		State:       resp.State,
	})
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
