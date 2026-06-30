package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"terraform-provider-passbolt/tools"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

var (
	_ resource.Resource                     = &passwordResource{}
	_ resource.ResourceWithConfigure        = &passwordResource{}
	_ resource.ResourceWithConfigValidators = &passwordResource{}
	_ resource.ResourceWithImportState      = &passwordResource{}
)

// NewPasswordResource returns a new instance of passwordResource as a Terraform resource.
func NewPasswordResource() resource.Resource {
	return &passwordResource{}
}

type passwordResource struct {
	client *tools.PassboltClient
}

const passwordImportSecretModeUnknownPrivateKey = "password_import_secret_mode_unknown"

type passwordPrivateStateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

type passwordModel struct {
	ID                 types.String   `tfsdk:"id"`
	Name               types.String   `tfsdk:"name"`
	Description        types.String   `tfsdk:"description"`
	Username           types.String   `tfsdk:"username"`
	URI                types.String   `tfsdk:"uri"`
	ShareGroup         types.String   `tfsdk:"share_group"`
	ShareGroups        []types.String `tfsdk:"share_groups"`
	FolderParent       types.String   `tfsdk:"folder_parent"`
	Password           types.String   `tfsdk:"password"`
	PasswordWO         types.String   `tfsdk:"password_wo"`
	PasswordWOVersion  types.Int64    `tfsdk:"password_wo_version"`
	MetadataType       types.String   `tfsdk:"metadata_type"`
	MetadataTypeActual types.String   `tfsdk:"metadata_type_actual"`
}

type passwordStringUpdateRequest struct {
	ID             string       `json:"id,omitempty"`
	ResourceTypeID string       `json:"resource_type_id,omitempty"`
	Name           string       `json:"name,omitempty"`
	Username       string       `json:"username,omitempty"`
	URI            *string      `json:"uri,omitempty"`
	Description    *string      `json:"description,omitempty"`
	Secrets        []api.Secret `json:"secrets,omitempty"`
}

type passwordMetadataPayload struct {
	ObjectType     string   `json:"object_type"`
	ResourceTypeID string   `json:"resource_type_id"`
	Name           string   `json:"name"`
	Username       string   `json:"username"`
	URIs           []string `json:"uris"`
	Description    *string  `json:"description,omitempty"`
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

func (r *passwordResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, passwordImportSecretModeUnknownPrivateKey, []byte(`true`))...)
}

func (r *passwordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password"
}

func (r *passwordResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("password"),
			path.MatchRoot("password_wo"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("password_wo"),
			path.MatchRoot("password_wo_version"),
		),
	}
}

func (r *passwordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a secret/password entry in Passbolt. Supports optional folder placement, group sharing, " +
			"Passbolt v4 and v5 encrypted metadata resources, and both legacy stateful passwords and write-only " +
			"password workflows.",
		Attributes: passwordResourceSchemaAttributes(),
	}
}

func passwordResourceSchemaAttributes() map[string]schema.Attribute {
	return mergePasswordAttributes(
		passwordResourceCoreAttributes(),
		passwordResourceSharingAttributes(),
		passwordResourceSecretAttributes(),
	)
}

func passwordResourceCoreAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The UUID of the Passbolt password/secret resource. Used for import and internal tracking.",
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
			Description: "The URI or URL where the secret is used (e.g., https://service.example.com).",
		},
		"folder_parent": schema.StringAttribute{
			Optional:    true,
			Description: "Name or UUID of an existing folder to place the secret in. Leave unset to place at top level.",
		},
		"metadata_type": schema.StringAttribute{
			Optional: true,
			Description: "Optional metadata format for this password. Use `v5` to create or migrate the password " +
				"to encrypted metadata, `v4` to force legacy cleartext metadata on create, or leave unset to use " +
				"the Passbolt server default without migrating existing passwords.",
			Validators: []validator.String{
				stringvalidator.OneOf(metadataTypeV4, metadataTypeV5, metadataTypeServerDefault),
			},
		},
		"metadata_type_actual": schema.StringAttribute{
			Computed:    true,
			Description: "Actual remote metadata format for this password: `v4` or `v5`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}

func passwordResourceSharingAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"share_group": schema.StringAttribute{
			Optional:    true,
			Description: "Name of the Passbolt group to share this secret with. Leave unset to keep private.",
		},
		"share_groups": schema.ListAttribute{
			ElementType: types.StringType,
			Optional:    true,
			Description: "List of Passbolt group names to share this secret with. Supports multiple group " +
				"shares. Takes precedence over `share_group`.",
		},
	}
}

func passwordResourceSecretAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"password": schema.StringAttribute{
			Optional:  true,
			Sensitive: true,
			Description: "Legacy secret input. Marked sensitive and masked in CLI output, but still stored in " +
				"Terraform state for drift detection. Use `password_wo` when you do not want Terraform to persist " +
				"the secret value.",
		},
		"password_wo": schema.StringAttribute{
			Optional:  true,
			Sensitive: true,
			WriteOnly: true,
			Description: "Write-only secret input. Terraform does not persist this value in plan or state files. " +
				"Set `password_wo_version` and increment it whenever you want to rotate the secret.",
		},
		"password_wo_version": schema.Int64Attribute{
			Optional: true,
			Description: "Version tracker for `password_wo`. Terraform stores this value in state so you can " +
				"trigger password rotation by incrementing it. Required when `password_wo` is configured.",
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
			},
		},
	}
}

func mergePasswordAttributes(groups ...map[string]schema.Attribute) map[string]schema.Attribute {
	attributes := make(map[string]schema.Attribute)

	for _, group := range groups {
		for key, attribute := range group {
			attributes[key] = attribute
		}
	}

	return attributes
}

func (r *passwordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var config passwordModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	resourceID, metadataTypeActual, err := createPassboltPasswordResource(
		ctx,
		r.client,
		folderID,
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		configuredPassword(config),
		plan.Description.ValueString(),
		desiredMetadataType(plan.MetadataType),
	)
	if err != nil {
		resp.Diagnostics.AddError("Cannot create resource", err.Error())

		return
	}

	plan.ID = types.StringValue(resourceID)
	plan.MetadataTypeActual = types.StringValue(metadataTypeActual)

	if len(plan.ShareGroups) > 0 {
		shareResourceWithGroups(ctx, r.client, plan.ShareGroups, resourceID, &resp.Diagnostics)
	} else if !plan.ShareGroup.IsUnknown() && !plan.ShareGroup.IsNull() && plan.ShareGroup.ValueString() != "" {
		shareResourceWithGroups(ctx, r.client, []types.String{plan.ShareGroup}, resourceID, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, buildManagedPasswordState(plan, config, types.StringValue(resourceID)))...)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, passwordImportSecretModeUnknownPrivateKey, nil)...)
}

// resolveFolderId can now match both name and UUID
func resolveFolderID(
	ctx context.Context,
	client *tools.PassboltClient,
	folder types.String) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if folder.IsUnknown() || folder.IsNull() {
		return "", diags
	}

	value := folder.ValueString()
	folders, err := getPassboltFolders(ctx, client, nil)
	if err != nil {
		diags.AddError("Cannot get folders", err.Error())

		return "", diags
	}

	for _, f := range folders {
		if f.ID == value || f.Name == value {
			return f.ID, diags
		}
	}

	diags.AddError("Folder not found", fmt.Sprintf("Folder with name or ID '%s' not found", value))

	return "", diags
}

func createPassboltPasswordResource(
	ctx context.Context,
	client *tools.PassboltClient,
	folderID string,
	name string,
	username string,
	uri string,
	password string,
	description string,
	metadataType string,
) (string, string, error) {
	actualType := passwordMetadataTypeForCreate(client, metadataType)

	switch actualType {
	case metadataTypeV5:
		resourceID, err := helper.CreateResourceGeneric(ctx, client.Client, "v5-default", folderID,
			map[string]any{
				"name":     name,
				"username": username,
				"uris":     []string{uri},
			},
			map[string]any{
				"password":    password,
				"description": description,
			},
		)

		return resourceID, actualType, err
	case metadataTypeV4:
		resourceID, err := helper.CreateResourceGeneric(ctx, client.Client, "password-and-description", folderID,
			map[string]any{
				"name":     name,
				"username": username,
				"uri":      uri,
			},
			map[string]any{
				"password":    password,
				"description": description,
			},
		)

		return resourceID, actualType, err
	default:
		resourceID, err := helper.CreateResource(
			ctx,
			client.Client,
			folderID,
			name,
			username,
			uri,
			password,
			description,
		)

		return resourceID, passwordMetadataTypeForCreate(client, metadataTypeServerDefault), err
	}
}

func passwordMetadataTypeForCreate(client *tools.PassboltClient, metadataType string) string {
	if metadataType == metadataTypeServerDefault {
		if client.Client.MetadataTypeSettings().DefaultResourceType == api.PassboltAPIVersionTypeV5 {
			return metadataTypeV5
		}

		return metadataTypeV4
	}

	return metadataType
}

func shareResourceWithGroups(
	ctx context.Context,
	client *tools.PassboltClient,
	groupNames []types.String,
	resourceID string,
	diags *diag.Diagnostics,
) {
	if len(groupNames) == 0 {
		return
	}

	groupsByName, groupErr := buildGroupNameMap(ctx, client, diags)
	if groupErr != nil {
		return
	}

	existingPerms, permErr := getExistingGroupPermissions(ctx, client, resourceID, diags)
	if permErr != nil {
		return
	}

	changes := make([]helper.ShareOperation, 0, len(groupNames))
	for _, name := range groupNames {
		groupName := name.ValueString()
		group, ok := groupsByName[groupName]
		if !ok {
			diags.AddError("Group not found", fmt.Sprintf("Group with name '%s' not found", groupName))

			continue
		}
		if existingPerms[group.ID] == 7 {
			continue
		}
		changes = append(changes, helper.ShareOperation{
			Type:  7,
			ARO:   "Group",
			AROID: group.ID,
		})
	}

	if len(changes) > 0 {
		if err := helper.ShareResource(ctx, client.Client, resourceID, changes); err != nil {
			diags.AddError("Cannot share resource", err.Error())
		}
	}
}

func buildGroupNameMap(
	ctx context.Context,
	client *tools.PassboltClient,
	diags *diag.Diagnostics,
) (map[string]api.Group, error) {
	groups, err := client.Client.GetGroups(ctx, nil)
	if err != nil {
		diags.AddError("Cannot get groups", err.Error())

		return nil, err
	}
	result := make(map[string]api.Group)
	for _, g := range groups {
		result[g.Name] = g
	}

	return result, nil
}

func getExistingGroupPermissions(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	diags *diag.Diagnostics,
) (map[string]int, error) {
	perms, err := client.Client.GetResourcePermissions(ctx, resourceID)
	if err != nil {
		diags.AddError("Cannot get existing permissions", err.Error())

		return nil, err
	}
	result := make(map[string]int)
	for _, p := range perms {
		if p.ARO == "Group" {
			result[p.AROForeignKey] = p.Type
		}
	}

	return result, nil
}

func buildPasswordState(
	ctx context.Context,
	client *tools.PassboltClient,
	id string,
	existing passwordModel,
	importedSecretModeUnknown bool,
) (passwordModel, diag.Diagnostics) {
	var state passwordModel
	var diags diag.Diagnostics

	resourceData, err := client.Client.GetResource(ctx, id)
	if err != nil {
		diags.AddError("Cannot read resource", err.Error())

		return state, diags
	}

	folderID, name, username, uri, password, description, err := helper.GetResource(ctx, client.Client, id)
	if err != nil {
		diags.AddError("Cannot read resource", err.Error())

		return state, diags
	}

	state.ID = types.StringValue(id)
	state.Name = types.StringValue(name)
	state.Username = types.StringValue(username)
	state.URI = types.StringValue(uri)
	state.Description = pickOptional(description)
	state.FolderParent = pickOptional(folderID)
	state.MetadataType = existing.MetadataType
	state.MetadataTypeActual = types.StringValue(actualMetadataTypeFromEncryptedMetadata(resourceData.Metadata))
	state.Password, state.PasswordWO, state.PasswordWOVersion = buildPasswordStateSecrets(
		existing,
		password,
		importedSecretModeUnknown,
	)
	state.ShareGroup = buildPasswordStateShareGroup(existing)
	state.ShareGroups = buildPasswordStateShareGroups(existing)

	return state, diags
}

func buildPasswordStateSecrets(
	existing passwordModel,
	actualPassword string,
	importedSecretModeUnknown bool,
) (types.String, types.String, types.Int64) {
	if usesWriteOnlyPassword(existing) {
		return types.StringNull(), types.StringNull(), existing.PasswordWOVersion
	}

	if existing.Password.IsNull() || existing.Password.IsUnknown() {
		if importedSecretModeUnknown {
			return types.StringNull(), types.StringNull(), types.Int64Null()
		}

		return pickOptional(actualPassword), types.StringNull(), types.Int64Null()
	}

	return pickPassword(actualPassword, existing.Password), types.StringNull(), types.Int64Null()
}

func buildPasswordStateShareGroup(existing passwordModel) types.String {
	if existing.ShareGroup.IsUnknown() || existing.ShareGroup.IsNull() || existing.ShareGroup.ValueString() == "" {
		return types.StringNull()
	}

	return existing.ShareGroup
}

func buildPasswordStateShareGroups(existing passwordModel) []types.String {
	if len(existing.ShareGroups) == 0 {
		return nil
	}

	return existing.ShareGroups
}

func pickPassword(actual string, existing types.String) types.String {
	if actual != "" {
		return types.StringValue(actual)
	}
	if existing.IsUnknown() || existing.IsNull() {
		return types.StringNull()
	}

	return existing
}

func pickOptional(value string) types.String {
	if value == "" {
		return types.StringNull()
	}

	return types.StringValue(value)
}

// Read retrieves the current state of the resource from Passbolt.
func (r *passwordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state passwordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	importedSecretModeUnknown := passwordImportSecretModeUnknown(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := buildPasswordState(ctx, r.client, state.ID.ValueString(), state, importedSecretModeUnknown)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *passwordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var config passwordModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	metadataTypeActual, diags := updateResourceFields(ctx, r, config, plan, state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)

		return
	}

	plan.MetadataTypeActual = types.StringValue(metadataTypeActual)
	resp.Diagnostics.Append(resp.State.Set(ctx, buildManagedPasswordState(plan, config, state.ID))...)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, passwordImportSecretModeUnknownPrivateKey, nil)...)
}

func passwordImportSecretModeUnknown(
	ctx context.Context,
	privateState passwordPrivateStateReader,
	diags *diag.Diagnostics,
) bool {
	value, privateDiags := privateState.GetKey(ctx, passwordImportSecretModeUnknownPrivateKey)
	diags.Append(privateDiags...)
	if privateDiags.HasError() {
		return false
	}

	return string(value) == "true"
}

func updateResourceFields(
	ctx context.Context,
	r *passwordResource,
	config passwordModel,
	plan passwordModel,
	state passwordModel,
) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	passwordValue, descriptionValue, secretDiags := resolveSecretUpdateInputs(ctx, r.client, config, plan, state)
	diags.Append(secretDiags...)
	if diags.HasError() {
		return "", diags
	}

	metadataTypeActual, err := ensurePassboltPasswordMetadataType(
		ctx,
		r.client,
		state.ID.ValueString(),
		desiredMetadataType(plan.MetadataType),
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		descriptionValue,
	)
	if err != nil {
		diags.AddError("Error updating resource metadata type", err.Error())

		return "", diags
	}

	err = updatePassboltPasswordResource(
		ctx,
		r.client,
		state.ID.ValueString(),
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		passwordStringChanged(plan.URI, state.URI),
		passwordValue,
		descriptionValue,
	)
	if err != nil {
		diags.AddError("Error updating resource", err.Error())

		return "", diags
	}

	diags.Append(moveResourceIfNeeded(ctx, r.client, plan, state)...)
	shareResourceIfNeeded(ctx, r.client, plan, state.ID.ValueString(), &diags)

	return metadataTypeActual, diags
}

func ensurePassboltPasswordMetadataType(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	metadataType string,
	name string,
	username string,
	uri string,
	description string,
) (string, error) {
	resourceData, resourceType, err := loadPasswordResourceType(ctx, client, resourceID)
	if err != nil {
		return "", err
	}

	actualType := actualMetadataTypeFromEncryptedMetadata(resourceData.Metadata)
	if metadataType == metadataTypeServerDefault || metadataType == actualType {
		return actualType, nil
	}

	if metadataType == metadataTypeV4 && actualType == metadataTypeV5 {
		return actualType, fmt.Errorf(
			"downgrading passwords from v5 encrypted metadata to v4 cleartext metadata is not supported",
		)
	}

	if metadataType == metadataTypeV5 && actualType == metadataTypeV4 {
		if err := upgradePassboltPasswordToV5(
			ctx,
			client,
			resourceData,
			resourceType,
			name,
			username,
			uri,
			description,
		); err != nil {
			return "", err
		}

		return metadataTypeV5, nil
	}

	return actualType, nil
}

func upgradePassboltPasswordToV5(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceData *api.Resource,
	resourceType *api.ResourceType,
	name string,
	username string,
	uri string,
	description string,
) error {
	if !client.Client.MetadataTypeSettings().AllowV4V5Upgrade {
		return fmt.Errorf("v4 to v5 metadata upgrade is disabled on this server")
	}

	if resourceData.Modified == nil {
		return fmt.Errorf(
			"resource %s cannot be upgraded because the API response has no modified timestamp",
			resourceData.ID,
		)
	}

	v5ResourceType, err := passwordV5ResourceTypeForUpgrade(ctx, client, resourceType)
	if err != nil {
		return err
	}

	request, err := buildPasswordMetadataUpgradeRequest(
		ctx,
		client,
		resourceData,
		v5ResourceType,
		name,
		username,
		uri,
		description,
	)
	if err != nil {
		return err
	}

	_, err = client.Client.DoCustomRequestV5(ctx, "POST", "/metadata/upgrade/resources.json", []metadataUpgradeRequest{
		request,
	}, nil)
	if err != nil {
		return fmt.Errorf("upgrading resource metadata to v5: %w", err)
	}

	return nil
}

func buildPasswordMetadataUpgradeRequest(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceData *api.Resource,
	v5ResourceType *api.ResourceType,
	name string,
	username string,
	uri string,
	description string,
) (metadataUpgradeRequest, error) {
	metadataKeyID, metadataKeyType, publicMetadataKey, err := client.Client.GetMetadataKey(ctx, false)
	if err != nil {
		return metadataUpgradeRequest{}, fmt.Errorf("get metadata key: %w", err)
	}

	metadata := passwordUpgradeMetadataPayload(v5ResourceType, name, username, uri, description)

	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return metadataUpgradeRequest{}, fmt.Errorf("marshal resource metadata: %w", err)
	}

	encryptedMetadata, err := client.Client.EncryptMetadata(publicMetadataKey, string(encodedMetadata))
	if err != nil {
		return metadataUpgradeRequest{}, err
	}

	return metadataUpgradeRequest{
		ID:              resourceData.ID,
		MetadataKeyID:   metadataKeyID,
		MetadataKeyType: metadataKeyType,
		Metadata:        encryptedMetadata,
		Modified:        resourceData.Modified.Format(time.RFC3339),
		ModifiedBy:      resourceData.ModifiedBy,
	}, nil
}

func passwordUpgradeMetadataPayload(
	v5ResourceType *api.ResourceType,
	name string,
	username string,
	uri string,
	description string,
) passwordMetadataPayload {
	metadata := passwordMetadataPayload{
		ObjectType:     api.PassboltObjectTypeResourceMetadata,
		ResourceTypeID: v5ResourceType.ID,
		Name:           name,
		Username:       username,
		URIs:           []string{uri},
	}

	if v5ResourceType.Slug == "v5-password-string" {
		metadata.Description = &description
	}

	return metadata
}

func passwordV5ResourceTypeForUpgrade(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceType *api.ResourceType,
) (*api.ResourceType, error) {
	return findPassboltResourceTypeBySlug(ctx, client, passwordV5ResourceTypeSlugForUpgrade(resourceType))
}

func passwordV5ResourceTypeSlugForUpgrade(resourceType *api.ResourceType) string {
	if resourceType.Slug == "password-string" {
		return "v5-password-string"
	}

	return "v5-default"
}

func findPassboltResourceTypeBySlug(
	ctx context.Context,
	client *tools.PassboltClient,
	slug string,
) (*api.ResourceType, error) {
	resourceTypes, err := client.Client.GetResourceTypes(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("getting resource types: %w", err)
	}

	for i := range resourceTypes {
		if resourceTypes[i].Slug == slug {
			return &resourceTypes[i], nil
		}
	}

	return nil, fmt.Errorf("cannot find resource type: %s", slug)
}

func updatePassboltPasswordResource(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	name string,
	username string,
	uri string,
	updateURI bool,
	password string,
	description string,
) error {
	resourceData, resourceType, err := loadPasswordResourceType(ctx, client, resourceID)
	if err != nil {
		return err
	}

	if isV4PasswordStringResource(resourceData, resourceType) {
		return updateV4PasswordStringResource(
			ctx,
			client,
			resourceData,
			resourceID,
			name,
			username,
			uri,
			updateURI,
			password,
			description,
		)
	}

	metadataUpdates, secretUpdates := passwordUpdateMaps(name, username, uri, updateURI, password, description)

	return helper.UpdateResourceGeneric(ctx, client.Client, resourceID, metadataUpdates, secretUpdates)
}

func loadPasswordResourceType(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
) (*api.Resource, *api.ResourceType, error) {
	resourceData, err := client.Client.GetResource(ctx, resourceID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting resource: %w", err)
	}

	resourceType, err := client.Client.GetResourceType(ctx, resourceData.ResourceTypeID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting resource type: %w", err)
	}

	return resourceData, resourceType, nil
}

func isV4PasswordStringResource(resourceData *api.Resource, resourceType *api.ResourceType) bool {
	return resourceData.Metadata == "" && resourceType.Slug == "password-string"
}

func updateV4PasswordStringResource(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceData *api.Resource,
	resourceID string,
	name string,
	username string,
	uri string,
	updateURI bool,
	password string,
	description string,
) error {
	secretData := password
	if secretData == "" {
		currentSecret, err := readDecryptedSecret(ctx, client, resourceID)
		if err != nil {
			return err
		}

		secretData = currentSecret
	}

	users, err := client.Client.GetUsers(ctx, &api.GetUsersOptions{
		FilterHasAccess: []string{resourceID},
	})
	if err != nil {
		return fmt.Errorf("getting users: %w", err)
	}

	secrets, err := encryptSecretDataForUsers(client.Client, users, secretData)
	if err != nil {
		return err
	}

	request := passwordStringUpdateRequest{
		ID:             resourceID,
		ResourceTypeID: resourceData.ResourceTypeID,
		Name:           name,
		Username:       username,
		Description:    &description,
		Secrets:        secrets,
	}
	if updateURI {
		request.URI = &uri
	}

	_, err = client.Client.DoCustomRequestV5(ctx, "PUT", "/resources/"+resourceID+".json", request, nil)
	if err != nil {
		return fmt.Errorf("updating v4 password-string resource: %w", err)
	}

	return nil
}

func readDecryptedSecret(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
) (string, error) {
	secret, err := client.Client.GetSecret(ctx, resourceID)
	if err != nil {
		return "", fmt.Errorf("getting secret: %w", err)
	}

	decryptedSecret, err := client.Client.DecryptMessage(secret.Data)
	if err != nil {
		return "", fmt.Errorf("decrypting secret: %w", err)
	}

	return decryptedSecret, nil
}

func encryptSecretDataForUsers(client *api.Client, users []api.User, secretData string) ([]api.Secret, error) {
	secrets := make([]api.Secret, 0, len(users))

	for _, user := range users {
		encryptedSecret, err := encryptSecretDataForUser(client, user, secretData)
		if err != nil {
			return nil, err
		}

		secrets = append(secrets, api.Secret{
			UserID: user.ID,
			Data:   encryptedSecret,
		})
	}

	return secrets, nil
}

func encryptSecretDataForUser(client *api.Client, user api.User, secretData string) (string, error) {
	if user.ID == client.GetUserID() {
		encryptedSecret, err := client.EncryptMessage(secretData)
		if err != nil {
			return "", fmt.Errorf("encrypting secret data for current user: %w", err)
		}

		return encryptedSecret, nil
	}

	publicKey, err := crypto.NewKeyFromArmored(user.GPGKey.ArmoredKey)
	if err != nil {
		return "", fmt.Errorf("getting public key for user %s: %w", user.ID, err)
	}

	encryptedSecret, err := client.EncryptMessageWithKey(publicKey, secretData)
	if err != nil {
		return "", fmt.Errorf("encrypting secret data for user %s: %w", user.ID, err)
	}

	return encryptedSecret, nil
}

func passwordUpdateMaps(
	name string,
	username string,
	uri string,
	updateURI bool,
	password string,
	description string,
) (map[string]any, map[string]any) {
	metadataUpdates := map[string]any{
		"name":        name,
		"username":    username,
		"description": description,
	}
	if updateURI {
		metadataUpdates["uri"] = uri
	}

	secretUpdates := map[string]any{
		"description": description,
	}

	if password != "" {
		secretUpdates["password"] = password
	}

	return metadataUpdates, secretUpdates
}

func passwordStringChanged(plan types.String, state types.String) bool {
	if plan.IsUnknown() || state.IsUnknown() {
		return true
	}
	if plan.IsNull() || state.IsNull() {
		return plan.IsNull() != state.IsNull()
	}

	return plan.ValueString() != state.ValueString()
}

func moveResourceIfNeeded(
	ctx context.Context,
	client *tools.PassboltClient,
	plan passwordModel,
	state passwordModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if plan.FolderParent.IsUnknown() || plan.FolderParent.ValueString() == state.FolderParent.ValueString() {
		return diags
	}

	newFolderID, folderDiags := resolveFolderID(ctx, client, plan.FolderParent)
	diags.Append(folderDiags...)
	if diags.HasError() {
		return diags
	}

	if err := client.Client.MoveResource(ctx, state.ID.ValueString(), newFolderID); err != nil {
		diags.AddError("Error moving resource to folder", err.Error())
	}

	return diags
}

func shareResourceIfNeeded(
	ctx context.Context,
	client *tools.PassboltClient,
	plan passwordModel,
	resourceID string,
	diags *diag.Diagnostics,
) {
	if len(plan.ShareGroups) > 0 {
		shareResourceWithGroups(ctx, client, plan.ShareGroups, resourceID, diags)

		return
	}

	if !plan.ShareGroup.IsUnknown() && !plan.ShareGroup.IsNull() && plan.ShareGroup.ValueString() != "" {
		shareResourceWithGroups(ctx, client, []types.String{plan.ShareGroup}, resourceID, diags)
	}
}

func configuredPassword(config passwordModel) string {
	if hasWriteOnlyPasswordConfig(config) {
		return config.PasswordWO.ValueString()
	}

	return config.Password.ValueString()
}

func passwordForUpdate(config, state passwordModel) string {
	if hasWriteOnlyPasswordConfig(config) {
		if passwordWOVersionChanged(config, state) {
			return config.PasswordWO.ValueString()
		}

		return ""
	}

	return config.Password.ValueString()
}

func resolveSecretUpdateInputs(
	ctx context.Context,
	client *tools.PassboltClient,
	config passwordModel,
	plan passwordModel,
	state passwordModel,
) (string, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	passwordValue := passwordForUpdate(config, state)
	descriptionValue := plan.Description.ValueString()

	if !hasWriteOnlyPasswordConfig(config) || passwordWOVersionChanged(config, state) {
		return passwordValue, descriptionValue, diags
	}

	currentPassword, currentDescription, err := readCurrentSecretValues(ctx, client, state.ID.ValueString())
	if err != nil {
		diags.AddError("Error reading current password for write-only update", err.Error())

		return "", "", diags
	}

	passwordValue = currentPassword

	if plan.Description.IsUnknown() {
		descriptionValue = currentDescription
	}

	return passwordValue, descriptionValue, diags
}

func readCurrentSecretValues(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
) (string, string, error) {
	folderID, name, username, uri, password, description, err := helper.GetResource(ctx, client.Client, resourceID)
	_ = folderID
	_ = name
	_ = username
	_ = uri

	return password, description, err
}

func buildManagedPasswordState(plan, config passwordModel, id types.String) passwordModel {
	state := plan
	state.ID = id
	state.PasswordWO = types.StringNull()

	if hasWriteOnlyPasswordConfig(config) {
		state.Password = types.StringNull()
		state.PasswordWOVersion = config.PasswordWOVersion

		return state
	}

	state.Password = config.Password
	state.PasswordWOVersion = types.Int64Null()

	return state
}

func hasWriteOnlyPasswordConfig(config passwordModel) bool {
	return !config.PasswordWO.IsNull() && !config.PasswordWO.IsUnknown()
}

func usesWriteOnlyPassword(state passwordModel) bool {
	return !state.PasswordWOVersion.IsNull() && !state.PasswordWOVersion.IsUnknown()
}

func passwordWOVersionChanged(config, state passwordModel) bool {
	if config.PasswordWOVersion.IsNull() || config.PasswordWOVersion.IsUnknown() {
		return false
	}

	if state.PasswordWOVersion.IsNull() || state.PasswordWOVersion.IsUnknown() {
		return true
	}

	return config.PasswordWOVersion.ValueInt64() != state.PasswordWOVersion.ValueInt64()
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
