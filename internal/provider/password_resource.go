package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	ID                types.String   `tfsdk:"id"`
	Name              types.String   `tfsdk:"name"`
	Description       types.String   `tfsdk:"description"`
	Username          types.String   `tfsdk:"username"`
	URI               types.String   `tfsdk:"uri"`
	ShareGroup        types.String   `tfsdk:"share_group"`
	ShareGroups       []types.String `tfsdk:"share_groups"`
	FolderParent      types.String   `tfsdk:"folder_parent"`
	Password          types.String   `tfsdk:"password"`
	PasswordWO        types.String   `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64    `tfsdk:"password_wo_version"`
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
			"and both legacy stateful passwords and write-only password workflows.",
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

	resourceID, err := helper.CreateResource(
		ctx,
		r.client.Client,
		folderID,
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		configuredPassword(config),
		plan.Description.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Cannot create resource", err.Error())

		return
	}

	plan.ID = types.StringValue(resourceID)

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
	folders, err := client.Client.GetFolders(ctx, nil)
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

	if diags := updateResourceFields(ctx, r, config, plan, state); diags.HasError() {
		resp.Diagnostics.Append(diags...)

		return
	}

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
) diag.Diagnostics {
	var diags diag.Diagnostics
	passwordValue, descriptionValue, secretDiags := resolveSecretUpdateInputs(ctx, r.client, config, plan, state)
	diags.Append(secretDiags...)
	if diags.HasError() {
		return diags
	}

	err := updatePassboltPasswordResource(
		ctx,
		r.client,
		state.ID.ValueString(),
		plan.Name.ValueString(),
		plan.Username.ValueString(),
		plan.URI.ValueString(),
		passwordValue,
		descriptionValue,
	)
	if err != nil {
		diags.AddError("Error updating resource", err.Error())

		return diags
	}

	diags.Append(moveResourceIfNeeded(ctx, r.client, plan, state)...)
	shareResourceIfNeeded(ctx, r.client, plan, state.ID.ValueString(), &diags)

	return diags
}

func updatePassboltPasswordResource(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	name string,
	username string,
	uri string,
	password string,
	description string,
) error {
	resourceData, resourceType, users, err := loadPasswordUpdateContext(ctx, client, resourceID)
	if err != nil {
		return err
	}

	updatedResource := api.Resource{
		ID:             resourceID,
		ResourceTypeID: resourceData.ResourceTypeID,
		Name:           resourceData.Name,
		Username:       resourceData.Username,
		URI:            resourceData.URI,
	}
	updatedResource.Name = name
	updatedResource.Username = username
	updatedResource.URI = uri

	if resourceType.Slug == "password-string" {
		updatedResource.Description = description
	}

	secretData, err := buildUpdatedPasswordSecretData(ctx, client, resourceID, resourceType.Slug, password, description)
	if err != nil {
		return err
	}

	updatedResource.Secrets, err = encryptSecretDataForUsers(client.Client, users, secretData)
	if err != nil {
		return err
	}

	_, err = client.Client.UpdateResource(ctx, resourceID, updatedResource)
	if err != nil {
		return fmt.Errorf("updating resource: %w", err)
	}

	return nil
}

func loadPasswordUpdateContext(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
) (*api.Resource, *api.ResourceType, []api.User, error) {
	resourceData, err := client.Client.GetResource(ctx, resourceID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getting resource: %w", err)
	}

	resourceType, err := client.Client.GetResourceType(ctx, resourceData.ResourceTypeID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getting resource type: %w", err)
	}

	users, err := client.Client.GetUsers(ctx, &api.GetUsersOptions{
		FilterHasAccess: []string{resourceID},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getting users: %w", err)
	}

	return resourceData, resourceType, users, nil
}

func buildUpdatedPasswordSecretData(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	resourceTypeSlug string,
	password string,
	description string,
) (string, error) {
	switch resourceTypeSlug {
	case "password-string":
		return passwordSecretData(ctx, client, resourceID, password)
	case "password-and-description":
		return passwordAndDescriptionSecretData(ctx, client, resourceID, password, description)
	case "password-description-totp":
		return passwordDescriptionTOTPSecretData(ctx, client, resourceID, password, description)
	default:
		return "", fmt.Errorf("unsupported passbolt resource type: %s", resourceTypeSlug)
	}
}

func passwordSecretData(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	password string,
) (string, error) {
	if password != "" {
		return password, nil
	}

	secret, err := client.Client.GetSecret(ctx, resourceID)
	if err != nil {
		return "", fmt.Errorf("getting secret: %w", err)
	}

	secretData, err := client.Client.DecryptMessage(secret.Data)
	if err != nil {
		return "", fmt.Errorf("decrypting secret: %w", err)
	}

	return secretData, nil
}

func passwordAndDescriptionSecretData(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	password string,
	description string,
) (string, error) {
	secretData := api.SecretDataTypePasswordAndDescription{
		Password:    password,
		Description: description,
	}

	if password == "" {
		oldSecret, err := readPasswordAndDescriptionSecret(ctx, client, resourceID)
		if err != nil {
			return "", err
		}

		secretData.Password = oldSecret.Password
	}

	marshaled, err := json.Marshal(&secretData)
	if err != nil {
		return "", fmt.Errorf("marshalling secret data: %w", err)
	}

	return string(marshaled), nil
}

func passwordDescriptionTOTPSecretData(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
	password string,
	description string,
) (string, error) {
	oldSecret, err := readPasswordDescriptionTOTPSecret(ctx, client, resourceID)
	if err != nil {
		return "", err
	}

	if password != "" {
		oldSecret.Password = password
	}

	oldSecret.Description = description

	marshaled, err := json.Marshal(&oldSecret)
	if err != nil {
		return "", fmt.Errorf("marshalling secret data: %w", err)
	}

	return string(marshaled), nil
}

func readPasswordAndDescriptionSecret(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
) (api.SecretDataTypePasswordAndDescription, error) {
	var secretData api.SecretDataTypePasswordAndDescription

	decryptedSecret, err := readDecryptedSecret(ctx, client, resourceID)
	if err != nil {
		return secretData, err
	}

	err = json.Unmarshal([]byte(decryptedSecret), &secretData)
	if err != nil {
		return secretData, fmt.Errorf("parsing decrypted secret data: %w", err)
	}

	return secretData, nil
}

func readPasswordDescriptionTOTPSecret(
	ctx context.Context,
	client *tools.PassboltClient,
	resourceID string,
) (api.SecretDataTypePasswordDescriptionTOTP, error) {
	var secretData api.SecretDataTypePasswordDescriptionTOTP

	decryptedSecret, err := readDecryptedSecret(ctx, client, resourceID)
	if err != nil {
		return secretData, err
	}

	err = json.Unmarshal([]byte(decryptedSecret), &secretData)
	if err != nil {
		return secretData, fmt.Errorf("parsing decrypted secret data: %w", err)
	}

	return secretData, nil
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

	encryptedSecret, err := client.EncryptMessageWithPublicKey(user.GPGKey.ArmoredKey, secretData)
	if err != nil {
		return "", fmt.Errorf("encrypting secret data for user %s: %w", user.ID, err)
	}

	return encryptedSecret, nil
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
