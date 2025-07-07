package provider

import (
	"context"
	"fmt"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type groupDataSource struct {
	client *tools.PassboltClient
}

type groupDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// NewGroupDataSource returns a new instance of the Passbolt group data source.
func NewGroupDataSource() datasource.DataSource {
	return &groupDataSource{}
}

func (d *groupDataSource) Configure(_ context.Context,
	req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*tools.PassboltClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Type",
			fmt.Sprintf("Expected *PassboltClient, got: %T", req.ProviderData))

		return
	}
	d.client = client
}

func (d *groupDataSource) Metadata(_ context.Context,
	req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetch a Passbolt group by name.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the group to look up.",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the group.",
			},
		},
	}
}

func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groups, err := d.client.Client.GetGroups(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get groups", err.Error())

		return
	}

	for _, g := range groups {
		if g.Name == config.Name.ValueString() {
			state := groupDataSourceModel{
				ID:   types.StringValue(g.ID),
				Name: types.StringValue(g.Name),
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

			return
		}
	}

	resp.Diagnostics.AddError("Group not found", fmt.Sprintf("No group found with name %q", config.Name.ValueString()))
}
