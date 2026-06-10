package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/passbolt/go-passbolt/api"

	"terraform-provider-passbolt/tools"
)

const passboltFolderMetadataObjectType = "PASSBOLT_FOLDER_METADATA"

type passboltFolderAPI struct {
	ID                string              `json:"id,omitempty"`
	Created           *api.Time           `json:"created,omitempty"`
	CreatedBy         string              `json:"created_by,omitempty"`
	Modified          *api.Time           `json:"modified,omitempty"`
	ModifiedBy        string              `json:"modified_by,omitempty"`
	Name              string              `json:"name,omitempty"`
	Permissions       []api.Permission    `json:"permissions,omitempty"`
	FolderParentID    string              `json:"folder_parent_id,omitempty"`
	Personal          bool                `json:"personal,omitempty"`
	ChildrenResources []api.Resource      `json:"children_resources,omitempty"`
	ChildrenFolders   []passboltFolderAPI `json:"children_folders,omitempty"`
	MetadataKeyID     string              `json:"metadata_key_id,omitempty"`
	MetadataKeyType   api.MetadataKeyType `json:"metadata_key_type,omitempty"`
	Metadata          string              `json:"metadata,omitempty"`
}

type passboltFolderMetadata struct {
	ObjectType  string  `json:"object_type"`
	Name        string  `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
}

type metadataUpgradeRequest struct {
	ID              string              `json:"id"`
	MetadataKeyID   string              `json:"metadata_key_id,omitempty"`
	MetadataKeyType api.MetadataKeyType `json:"metadata_key_type"`
	Metadata        string              `json:"metadata"`
	Modified        string              `json:"modified"`
	ModifiedBy      string              `json:"modified_by"`
}

func getPassboltFolders(
	ctx context.Context,
	client *tools.PassboltClient,
	opts *api.GetFoldersOptions,
) ([]api.Folder, error) {
	msg, err := client.Client.DoCustomRequestV5(ctx, "GET", "/folders.json", nil, opts)
	if err != nil {
		return nil, err
	}

	var rawFolders []passboltFolderAPI
	if err := json.Unmarshal(msg.Body, &rawFolders); err != nil {
		return nil, err
	}

	folders := make([]api.Folder, 0, len(rawFolders))
	for _, rawFolder := range rawFolders {
		folder, err := normalizePassboltFolder(ctx, client.Client, rawFolder)
		if err != nil {
			return nil, err
		}

		folders = append(folders, folder)
	}

	return folders, nil
}

func getPassboltFolder(
	ctx context.Context,
	client *tools.PassboltClient,
	folderID string,
) (passboltFolderAPI, api.Folder, error) {
	msg, err := client.Client.DoCustomRequestV5(ctx, "GET", "/folders/"+folderID+".json", nil, nil)
	if err != nil {
		return passboltFolderAPI{}, api.Folder{}, err
	}

	var rawFolder passboltFolderAPI
	if err := json.Unmarshal(msg.Body, &rawFolder); err != nil {
		return passboltFolderAPI{}, api.Folder{}, err
	}

	folder, err := normalizePassboltFolder(ctx, client.Client, rawFolder)
	if err != nil {
		return passboltFolderAPI{}, api.Folder{}, err
	}

	return rawFolder, folder, nil
}

func normalizePassboltFolder(
	ctx context.Context,
	client *api.Client,
	rawFolder passboltFolderAPI,
) (api.Folder, error) {
	name := rawFolder.Name
	if rawFolder.Metadata != "" {
		metadata, err := decryptPassboltFolderMetadata(ctx, client, rawFolder)
		if err != nil {
			return api.Folder{}, fmt.Errorf("decrypting folder metadata for %s: %w", rawFolder.ID, err)
		}

		name = metadata.Name
	}

	children := make([]api.Folder, 0, len(rawFolder.ChildrenFolders))
	for _, rawChild := range rawFolder.ChildrenFolders {
		child, err := normalizePassboltFolder(ctx, client, rawChild)
		if err != nil {
			return api.Folder{}, err
		}

		children = append(children, child)
	}

	return api.Folder{
		ID:                rawFolder.ID,
		Created:           rawFolder.Created,
		CreatedBy:         rawFolder.CreatedBy,
		Modified:          rawFolder.Modified,
		ModifiedBy:        rawFolder.ModifiedBy,
		Name:              name,
		Permissions:       rawFolder.Permissions,
		FolderParentID:    rawFolder.FolderParentID,
		Personal:          rawFolder.Personal,
		ChildrenResources: rawFolder.ChildrenResources,
		ChildrenFolders:   children,
	}, nil
}

func decryptPassboltFolderMetadata(
	ctx context.Context,
	client *api.Client,
	folder passboltFolderAPI,
) (passboltFolderMetadata, error) {
	metadataKey, metadataKeyCacheID, err := folderMetadataPrivateKey(ctx, client, folder)
	if err != nil {
		return passboltFolderMetadata{}, err
	}

	decrypted, err := client.DecryptMetadataWithKeyID(metadataKeyCacheID, metadataKey, folder.Metadata)
	if err != nil {
		return passboltFolderMetadata{}, err
	}

	var metadata passboltFolderMetadata
	if err := json.Unmarshal([]byte(decrypted), &metadata); err != nil {
		return passboltFolderMetadata{}, err
	}

	if metadata.ObjectType != "" && metadata.ObjectType != passboltFolderMetadataObjectType {
		return passboltFolderMetadata{}, fmt.Errorf("unexpected folder metadata object_type %q", metadata.ObjectType)
	}

	return metadata, nil
}

func folderMetadataPrivateKey(
	ctx context.Context,
	client *api.Client,
	folder passboltFolderAPI,
) (*crypto.Key, string, error) {
	if folder.MetadataKeyType == api.MetadataKeyTypeUserKey {
		key, err := client.GetUserPrivateKeyCopy()
		if err != nil {
			return nil, "", fmt.Errorf("get user private key: %w", err)
		}

		return key, "user-key:" + key.GetFingerprint(), nil
	}

	key, err := client.GetDecryptedMetadataKeyCached(ctx, folder.MetadataKeyID)
	if err != nil {
		return nil, "", fmt.Errorf("get shared metadata key: %w", err)
	}

	return key, folder.MetadataKeyID, nil
}

func createPassboltFolder(
	ctx context.Context,
	client *tools.PassboltClient,
	folderParentID string,
	name string,
	metadataType string,
) (api.Folder, string, error) {
	actualType := folderMetadataTypeForCreate(client, metadataType)
	if err := validateFolderMetadataTypeCreation(client, actualType); err != nil {
		return api.Folder{}, "", err
	}

	if actualType == metadataTypeV4 {
		folder, err := client.Client.CreateFolder(ctx, api.Folder{
			FolderParentID: folderParentID,
			Name:           name,
			Personal:       false,
		})
		if err != nil {
			return api.Folder{}, "", err
		}

		return *folder, actualType, nil
	}

	metadataKeyID, metadataKeyType, encryptedMetadata, err := encryptFolderMetadata(ctx, client.Client, name)
	if err != nil {
		return api.Folder{}, "", err
	}

	body := passboltFolderAPI{
		FolderParentID:  folderParentID,
		MetadataKeyID:   metadataKeyID,
		MetadataKeyType: metadataKeyType,
		Metadata:        encryptedMetadata,
	}

	msg, err := client.Client.DoCustomRequestV5(ctx, "POST", "/folders.json", body, nil)
	if err != nil {
		return api.Folder{}, "", err
	}

	var rawFolder passboltFolderAPI
	if err := json.Unmarshal(msg.Body, &rawFolder); err != nil {
		return api.Folder{}, "", err
	}

	folder, err := normalizePassboltFolder(ctx, client.Client, rawFolder)
	if err != nil {
		return api.Folder{}, "", err
	}

	return folder, actualType, nil
}

func folderMetadataTypeForCreate(client *tools.PassboltClient, metadataType string) string {
	if metadataType == metadataTypeServerDefault {
		if client.Client.MetadataTypeSettings().DefaultFolderType == api.PassboltAPIVersionTypeV5 {
			return metadataTypeV5
		}

		return metadataTypeV4
	}

	return metadataType
}

func validateFolderMetadataTypeCreation(client *tools.PassboltClient, metadataType string) error {
	settings := client.Client.MetadataTypeSettings()
	if metadataType == metadataTypeV5 && !settings.AllowCreationOfV5Folders {
		return fmt.Errorf("creation of V5 folders is disabled on this server")
	}

	if metadataType == metadataTypeV4 && !settings.AllowCreationOfV4Folders {
		return fmt.Errorf("creation of V4 folders is disabled on this server")
	}

	return nil
}

func updatePassboltFolder(
	ctx context.Context,
	client *tools.PassboltClient,
	folderID string,
	name string,
	metadataType string,
) (string, error) {
	rawFolder, _, err := getPassboltFolder(ctx, client, folderID)
	if err != nil {
		return "", err
	}

	rawFolder, actualType, err := ensurePassboltFolderMetadataType(ctx, client, rawFolder, name, metadataType)
	if err != nil {
		return actualType, err
	}

	if actualType == metadataTypeV5 {
		if err := updatePassboltFolderV5(ctx, client, rawFolder, name); err != nil {
			return "", err
		}

		return metadataTypeV5, nil
	}

	_, err = client.Client.UpdateFolder(ctx, folderID, api.Folder{Name: name})
	if err != nil {
		return "", err
	}

	return metadataTypeV4, nil
}

func ensurePassboltFolderMetadataType(
	ctx context.Context,
	client *tools.PassboltClient,
	rawFolder passboltFolderAPI,
	name string,
	metadataType string,
) (passboltFolderAPI, string, error) {
	actualType := actualMetadataTypeFromEncryptedMetadata(rawFolder.Metadata)
	if metadataType == metadataTypeServerDefault || metadataType == actualType {
		return rawFolder, actualType, nil
	}

	if metadataType == metadataTypeV4 && actualType == metadataTypeV5 {
		return rawFolder, actualType, fmt.Errorf(
			"downgrading folders from v5 encrypted metadata to v4 cleartext metadata is not supported",
		)
	}

	if metadataType != metadataTypeV5 || actualType != metadataTypeV4 {
		return rawFolder, actualType, nil
	}

	if err := upgradePassboltFolderToV5(ctx, client, rawFolder, name); err != nil {
		return passboltFolderAPI{}, "", err
	}

	upgradedFolder, _, err := getPassboltFolder(ctx, client, rawFolder.ID)
	if err != nil {
		return passboltFolderAPI{}, "", err
	}

	return upgradedFolder, metadataTypeV5, nil
}

func updatePassboltFolderV5(
	ctx context.Context,
	client *tools.PassboltClient,
	folder passboltFolderAPI,
	name string,
) error {
	metadata, err := decryptPassboltFolderMetadata(ctx, client.Client, folder)
	if err != nil {
		return err
	}

	metadata.Name = name
	if metadata.ObjectType == "" {
		metadata.ObjectType = passboltFolderMetadataObjectType
	}

	usePersonalKey := folder.MetadataKeyType == api.MetadataKeyTypeUserKey
	metadataKeyID, metadataKeyType, encryptedMetadata, err := encryptFolderMetadataPayload(
		ctx,
		client.Client,
		metadata,
		usePersonalKey,
	)
	if err != nil {
		return err
	}

	body := passboltFolderAPI{
		MetadataKeyID:   metadataKeyID,
		MetadataKeyType: metadataKeyType,
		Metadata:        encryptedMetadata,
	}

	_, err = client.Client.DoCustomRequestV5(ctx, "PUT", "/folders/"+folder.ID+".json", body, nil)
	if err != nil {
		return fmt.Errorf("updating v5 folder metadata: %w", err)
	}

	return nil
}

func upgradePassboltFolderToV5(
	ctx context.Context,
	client *tools.PassboltClient,
	folder passboltFolderAPI,
	name string,
) error {
	if !client.Client.MetadataTypeSettings().AllowV4V5Upgrade {
		return fmt.Errorf("v4 to v5 metadata upgrade is disabled on this server")
	}

	if folder.Modified == nil {
		return fmt.Errorf("folder %s cannot be upgraded because the API response has no modified timestamp", folder.ID)
	}

	metadataKeyID, metadataKeyType, encryptedMetadata, err := encryptFolderMetadata(ctx, client.Client, name)
	if err != nil {
		return err
	}

	body := []metadataUpgradeRequest{{
		ID:              folder.ID,
		MetadataKeyID:   metadataKeyID,
		MetadataKeyType: metadataKeyType,
		Metadata:        encryptedMetadata,
		Modified:        folder.Modified.Format(time.RFC3339),
		ModifiedBy:      folder.ModifiedBy,
	}}

	_, err = client.Client.DoCustomRequestV5(ctx, "POST", "/metadata/upgrade/folders.json", body, nil)
	if err != nil {
		return fmt.Errorf("upgrading folder metadata to v5: %w", err)
	}

	return nil
}

func encryptFolderMetadata(
	ctx context.Context,
	client *api.Client,
	name string,
) (string, api.MetadataKeyType, string, error) {
	return encryptFolderMetadataPayload(ctx, client, passboltFolderMetadata{
		ObjectType: passboltFolderMetadataObjectType,
		Name:       name,
	}, false)
}

func encryptFolderMetadataPayload(
	ctx context.Context,
	client *api.Client,
	metadata passboltFolderMetadata,
	usePersonalKey bool,
) (string, api.MetadataKeyType, string, error) {
	metadataKeyID, metadataKeyType, publicMetadataKey, err := client.GetMetadataKey(ctx, usePersonalKey)
	if err != nil {
		return "", "", "", fmt.Errorf("get metadata key: %w", err)
	}

	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal folder metadata: %w", err)
	}

	encryptedMetadata, err := client.EncryptMetadata(publicMetadataKey, string(encodedMetadata))
	if err != nil {
		return "", "", "", err
	}

	return metadataKeyID, metadataKeyType, encryptedMetadata, nil
}
