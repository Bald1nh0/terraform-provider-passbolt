package provider

import (
	"context"
	"fmt"

	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
)

var (
	_ datasource.DataSource              = &userDataSource{}
	_ datasource.DataSourceWithConfigure = &userDataSource{}
)

// NewUserDataSource returns a Terraform data source for Passbolt users.
func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

type userDataSource struct {
	client *tools.PassboltClient
}

type userDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Username  types.String `tfsdk:"username"`
	Role      types.String `tfsdk:"role"`
	FirstName types.String `tfsdk:"first_name"`
	LastName  types.String `tfsdk:"last_name"`
}

func (d *userDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data Type",
			fmt.Sprintf("Expected *PassboltClient, got: %T",
				req.ProviderData))

		return
	}
	d.client = client
}

func (d *userDataSource) Metadata(_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username (email address) to look up.",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "User ID (UUID).",
			},
			"role": schema.StringAttribute{
				Computed:    true,
				Description: "Role of the user: admin or user.",
			},
			"first_name": schema.StringAttribute{
				Computed:    true,
				Description: "First name of the user.",
			},
			"last_name": schema.StringAttribute{
				Computed:    true,
				Description: "Last name of the user.",
			},
		},
	}
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, err := d.client.Client.GetUsers(ctx, &api.GetUsersOptions{
		FilterSearch: config.Username.ValueString(),
	})
	if err != nil || len(users) == 0 {
		resp.Diagnostics.AddError("User not found",
			fmt.Sprintf("Could not find user with username: %s",
				config.Username.ValueString()))

		return
	}

	user := users[0]

	data := userDataSourceModel{
		ID:        types.StringValue(user.ID),
		Username:  types.StringValue(user.Username),
		Role:      types.StringValue(user.Role.Name),
		FirstName: types.StringValue(user.Profile.FirstName),
		LastName:  types.StringValue(user.Profile.LastName),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
