package provider

import (
	"context"
	"fmt"
	"strings"

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
	ID              types.String `tfsdk:"id"`
	Username        types.String `tfsdk:"username"`
	IncludeInactive types.Bool   `tfsdk:"include_inactive"`
	Active          types.Bool   `tfsdk:"active"`
	Role            types.String `tfsdk:"role"`
	FirstName       types.String `tfsdk:"first_name"`
	LastName        types.String `tfsdk:"last_name"`
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
		Description: "Looks up an existing non-deleted Passbolt user by exact username (email address). " +
			"By default, only active users are returned.",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				Required: true,
				Description: "Exact username (email address) to look up. " +
					"The user must already be active unless include_inactive is enabled.",
			},
			"include_inactive": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "When true, allows returning an inactive, non-deleted Passbolt user. " +
					"This only affects the data source lookup. To skip inactive users during group " +
					"membership apply, use the returned ID as a regular passbolt_group member with " +
					"ignore_inactive_members enabled.",
			},
			"active": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the Passbolt user has completed activation.",
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
	if err != nil {
		resp.Diagnostics.AddError("User not found",
			fmt.Sprintf("Could not find user with username: %s",
				config.Username.ValueString()))

		return
	}

	includeInactive := config.IncludeInactive.ValueBool()
	user, err := userByUsername(users, config.Username.ValueString(), includeInactive)
	if err != nil {
		resp.Diagnostics.AddError("User not found", err.Error())

		return
	}

	data := userDataSourceModel{
		ID:              types.StringValue(user.ID),
		Username:        types.StringValue(user.Username),
		IncludeInactive: types.BoolValue(includeInactive),
		Active:          types.BoolValue(user.Active),
		Role:            types.StringValue(userRoleName(user)),
		FirstName:       types.StringValue(userFirstName(user)),
		LastName:        types.StringValue(userLastName(user)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type userLookupResult struct {
	active      *api.User
	inactive    *api.User
	sawDeleted  bool
	sawInactive bool
}

func userByUsername(users []api.User, username string, includeInactive bool) (*api.User, error) {
	result := findUserByUsername(users, username, includeInactive)
	if result.active != nil {
		return result.active, nil
	}
	if includeInactive && result.inactive != nil {
		return result.inactive, nil
	}

	return nil, userLookupError(username, includeInactive, result)
}

func findUserByUsername(users []api.User, username string, includeInactive bool) userLookupResult {
	var result userLookupResult
	for _, user := range users {
		if !strings.EqualFold(user.Username, username) {
			continue
		}

		if user.Deleted {
			result.sawDeleted = true

			continue
		}

		if !user.Active {
			result.sawInactive = true
			if includeInactive && result.inactive == nil {
				candidate := user
				result.inactive = &candidate
			}

			continue
		}

		candidate := user
		result.active = &candidate

		return result
	}

	return result
}

func userLookupError(username string, includeInactive bool, result userLookupResult) error {
	if result.sawDeleted {
		return fmt.Errorf("user %s exists in Passbolt but is deleted", username)
	}

	if result.sawInactive {
		return fmt.Errorf("user %s exists but is not active in Passbolt", username)
	}

	if includeInactive {
		return fmt.Errorf("could not find non-deleted Passbolt user with username: %s", username)
	}

	return fmt.Errorf("could not find active Passbolt user with username: %s", username)
}

func userRoleName(user *api.User) string {
	if user.Role == nil {
		return ""
	}

	return user.Role.Name
}

func userFirstName(user *api.User) string {
	if user.Profile == nil {
		return ""
	}

	return user.Profile.FirstName
}

func userLastName(user *api.User) string {
	if user.Profile == nil {
		return ""
	}

	return user.Profile.LastName
}
