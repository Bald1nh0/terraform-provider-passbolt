package provider

import (
	"context"
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
	_ resource.Resource              = &userResource{}
	_ resource.ResourceWithConfigure = &userResource{}
)

// NewUserResource returns a Terraform resource for managing Passbolt users.
func NewUserResource() resource.Resource {
	return &userResource{}
}

type userResource struct {
	client *tools.PassboltClient
}

type userModel struct {
	ID        types.String `tfsdk:"id"`
	Username  types.String `tfsdk:"username"`
	FirstName types.String `tfsdk:"first_name"`
	LastName  types.String `tfsdk:"last_name"`
	Role      types.String `tfsdk:"role"`
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data Type",
			fmt.Sprintf("Expected *PassboltClient, got %T",
				req.ProviderData))

		return
	}
	r.client = client
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the user.",
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username (email address).",
			},
			"first_name": schema.StringAttribute{
				Required:    true,
				Description: "First name of the user (required by API, not returned in read).",
			},
			"last_name": schema.StringAttribute{
				Required:    true,
				Description: "Last name of the user (required by API, not returned in read).",
			},
			"role": schema.StringAttribute{
				Required:    true,
				Description: "Role name: 'admin' or 'user'.",
			},
		},
	}
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := helper.CreateUser(ctx, r.client.Client,
		plan.Role.ValueString(),
		plan.Username.ValueString(),
		plan.FirstName.ValueString(),
		plan.LastName.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error creating user", err.Error())

		return
	}

	users, err := r.client.Client.GetUsers(ctx, &api.GetUsersOptions{
		FilterSearch: plan.Username.ValueString(),
	})
	if err != nil || len(users) == 0 {
		resp.Diagnostics.AddError("Error fetching created user", fmt.Sprintf("Failed to fetch user by email: %s", err))

		return
	}

	user := users[0]
	plan.ID = types.StringValue(user.ID)
	plan.Username = types.StringValue(user.Username)
	plan.Role = types.StringValue(user.Role.Name)

	// Keep First/Last name from plan — not returned from API
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, err := r.client.Client.GetUsers(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching users", err.Error())

		return
	}

	var user *api.User
	for _, u := range users {
		if u.ID == state.ID.ValueString() {
			user = &u

			break
		}
	}

	if user == nil {
		resp.State.RemoveResource(ctx)

		return
	}

	state.Username = types.StringValue(user.Username)
	state.Role = types.StringValue(user.Role.Name)

	// Keep First/Last from original state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := helper.UpdateUser(ctx, r.client.Client,
		state.ID.ValueString(),
		plan.Role.ValueString(),
		plan.FirstName.ValueString(),
		plan.LastName.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating user", err.Error())

		return
	}

	plan.ID = state.ID

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := helper.DeleteUser(ctx, r.client.Client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting user", err.Error())
	}
}
