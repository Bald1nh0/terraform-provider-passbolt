package provider

import (
	"context"
	"fmt"
	"os"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
)

var (
	_ provider.Provider = &passboltProvider{}
)

type passboltProvider struct {
	version string
}

type passboltProviderModel struct {
	URL  types.String `tfsdk:"base_url"`
	KEY  types.String `tfsdk:"private_key"`
	PASS types.String `tfsdk:"passphrase"`
}

// New returns a Terraform provider implementation for Passbolt.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &passboltProvider{version: version}
	}
}

func (p *passboltProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "passbolt"
	resp.Version = p.version
}

func (p *passboltProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Required: true,
				Description: "Base URL for the Passbolt instance. " +
					"Can also be provided via `PASSBOLT_URL` environment variable.",
			},
			"private_key": schema.StringAttribute{
				Required: true,
				Description: "ASCII-armored PGP Private key of Passbolt user. " +
					"Can also be provided via `PASSBOLT_KEY` env var.",
			},
			"passphrase": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Passphrase for the user's private key. " +
					"Can also be provided via `PASSBOLT_PASS` environment variable.",
			},
		},
	}
}

func (p *passboltProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var config passboltProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := resolveStringAttr(ctx, config.URL, "url", "PASSBOLT_URL", &resp.Diagnostics)
	key := resolveStringAttr(ctx, config.KEY, "private_key", "PASSBOLT_KEY", &resp.Diagnostics)
	pass := resolveStringAttr(ctx, config.PASS, "passphrase", "PASSBOLT_PASS", &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := api.NewClient(nil, "", url, key, pass)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to passbolt", "Client Error: "+err.Error())

		return
	}

	passboltClient := tools.PassboltClient{
		Client:     client,
		URL:        url,
		Password:   pass,
		PrivateKey: key,
	}

	if err := tools.Login(ctx, &passboltClient); err != nil {
		resp.Diagnostics.AddError("Login failed", err.Error())

		return
	}

	resp.DataSourceData = &passboltClient
	resp.ResourceData = &passboltClient
}

func resolveStringAttr(
	_ context.Context,
	val types.String,
	attrName string,
	envVar string,
	diags *diag.Diagnostics,
) string {
	if val.IsUnknown() {
		diags.AddAttributeError(
			path.Root(attrName),
			"Unknown "+attrName,
			fmt.Sprintf("The %s value is unknown and cannot be used yet.", attrName),
		)

		return ""
	}

	if val.IsNull() {
		val = types.StringValue(os.Getenv(envVar))
	}

	result := val.ValueString()

	if result == "" {
		diags.AddAttributeError(
			path.Root(attrName),
			"Missing "+attrName,
			fmt.Sprintf("No %s value was provided in configuration or environment variable %s.", attrName, envVar),
		)
	}

	return result
}

func (p *passboltProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewFoldersDataSource,
		NewPasswordDataSource,
	}
}

func (p *passboltProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFolderResource,
		NewPasswordResource,
		NewFolderPermissionResource,
	}
}
