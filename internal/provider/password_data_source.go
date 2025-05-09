package provider

import (
	"context"
	"fmt"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/helper"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &passwordDataSource{}
	_ datasource.DataSourceWithConfigure = &passwordDataSource{}
)

// NewPasswordDataSource is a helper function to simplify the provider implementation.
func NewPasswordDataSource() datasource.DataSource {
	return &passwordDataSource{}
}

// passwordDataSource is the data source implementation.
type passwordDataSource struct {
	client *tools.PassboltClient
}

type passwordDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Username       types.String `tfsdk:"username"`
	Uri            types.String `tfsdk:"uri"`
	FolderParentID types.String `tfsdk:"folder_parent_id"`
	Password       types.String `tfsdk:"password"`
}

// Configure adds the provider configured client to the data source.
func (d *passwordDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *passboltClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

// Metadata returns the data source type name.
func (d *passwordDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password"
}

// Schema defines the schema for the data source.
func (d *passwordDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetch a password/secret from Passbolt by its ID. Useful for lookups in cross-team automation or outputting secrets to other modules. Returns all metadata and the decrypted secret value.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "The Passbolt resource UUID (you can get it from the Passbolt UI or list of resources).",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the password/secret in Passbolt.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description field for the secret.",
			},
			"username": schema.StringAttribute{
				Computed:    true,
				Description: "Username/login for this secret (if set).",
			},
			"uri": schema.StringAttribute{
				Computed:    true,
				Description: "URI where the password is used, e.g., a service address.",
			},
			"folder_parent_id": schema.StringAttribute{
				Computed:    true,
				Description: "Parent folder's UUID containing this password/secret.",
			},
			"password": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The **actual secret value** (Sensitive, will not be displayed in UI/logs).",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *passwordDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data passwordDataSourceModel
	diag := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diag...)

	folderParentID, name, username, uri, password, description, err := helper.GetResource(d.client.Context, d.client.Client, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read resource "+data.ID.ValueString(), err.Error(),
		)
		return
	}

	data.Name = types.StringValue(name)
	data.Description = types.StringValue(description)
	data.Uri = types.StringValue(uri)
	data.Username = types.StringValue(username)
	data.FolderParentID = types.StringValue(folderParentID)
	data.Password = types.StringValue(password)

	// Set state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
