package provider

import (
	"context"
	"fmt"
	stdpath "path"
	"sort"
	"strings"
	"terraform-provider-passbolt/tools"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
)

func resolveFolderReference(
	ctx context.Context,
	client *tools.PassboltClient,
	folder types.String,
) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if folder.IsUnknown() || folder.IsNull() {
		return "", diags
	}

	value := strings.TrimSpace(folder.ValueString())
	if value == "" {
		return "", diags
	}

	folders, err := client.Client.GetFolders(ctx, nil)
	if err != nil {
		diags.AddError("Cannot get folders", err.Error())

		return "", diags
	}

	folderID, err := resolveFolderReferenceValue(folders, value)
	if err != nil {
		diags.AddError("Folder not found", err.Error())

		return "", diags
	}

	return folderID, diags
}

func resolveFolderReferenceValue(folders []api.Folder, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	for _, folder := range folders {
		if folder.ID == value {
			return folder.ID, nil
		}
	}

	if strings.HasPrefix(value, "/") {
		return resolveFolderReferenceByPath(folders, value)
	}

	return resolveFolderReferenceByName(folders, value)
}

func resolveFolderReferenceByPath(folders []api.Folder, value string) (string, error) {
	normalizedPath, err := normalizeFolderPath(value)
	if err != nil {
		return "", err
	}

	pathsByID, err := buildFolderPathIndex(folders)
	if err != nil {
		return "", err
	}

	for folderID, folderPath := range pathsByID {
		if folderPath == normalizedPath {
			return folderID, nil
		}
	}

	return "", fmt.Errorf("folder with path %q not found", normalizedPath)
}

func resolveFolderReferenceByName(folders []api.Folder, value string) (string, error) {
	matches := make([]api.Folder, 0, 1)
	for _, folder := range folders {
		if folder.Name == value {
			matches = append(matches, folder)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("folder with name %q not found", value)
	case 1:
		return matches[0].ID, nil
	}

	pathsByID, err := buildFolderPathIndex(folders)
	if err != nil {
		return "", err
	}

	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		folderPath, ok := pathsByID[match.ID]
		if !ok {
			return "", fmt.Errorf("folder path for %q could not be resolved", match.ID)
		}

		paths = append(paths, folderPath)
	}
	sort.Strings(paths)

	return "", fmt.Errorf(
		"folder name %q is ambiguous; use a UUID or absolute path instead. Matches: %s",
		value,
		strings.Join(paths, ", "),
	)
}

func buildFolderPathIndex(folders []api.Folder) (map[string]string, error) {
	foldersByID := make(map[string]api.Folder, len(folders))
	for _, folder := range folders {
		foldersByID[folder.ID] = folder
	}

	pathsByID := make(map[string]string, len(folders))
	visiting := make(map[string]bool, len(folders))

	for _, folder := range folders {
		if _, err := buildFolderPath(folder.ID, foldersByID, pathsByID, visiting); err != nil {
			return nil, err
		}
	}

	return pathsByID, nil
}

func buildFolderPath(
	folderID string,
	foldersByID map[string]api.Folder,
	pathsByID map[string]string,
	visiting map[string]bool,
) (string, error) {
	if folderPath, ok := pathsByID[folderID]; ok {
		return folderPath, nil
	}

	folder, ok := foldersByID[folderID]
	if !ok {
		return "", fmt.Errorf("folder with id %q not found while building folder paths", folderID)
	}

	if visiting[folderID] {
		return "", fmt.Errorf("cycle detected while building path for folder %q", folderID)
	}

	visiting[folderID] = true
	defer delete(visiting, folderID)

	if folder.FolderParentID == "" {
		folderPath := "/" + folder.Name
		pathsByID[folderID] = folderPath

		return folderPath, nil
	}

	parentPath, err := buildFolderPath(folder.FolderParentID, foldersByID, pathsByID, visiting)
	if err != nil {
		return "", fmt.Errorf("cannot build path for folder %q: %w", folder.Name, err)
	}

	folderPath := parentPath + "/" + folder.Name
	pathsByID[folderID] = folderPath

	return folderPath, nil
}

func normalizeFolderPath(value string) (string, error) {
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("folder path %q must start with '/'", value)
	}

	normalized := stdpath.Clean(value)
	if normalized == "." || normalized == "/" {
		return "", fmt.Errorf("folder path %q must point to a folder", value)
	}

	return normalized, nil
}

func reconcileFolderParentReference(existing types.String, remoteParentID string, folders []api.Folder) types.String {
	if remoteParentID == "" {
		return types.StringNull()
	}

	if existing.IsUnknown() || existing.IsNull() {
		return types.StringValue(remoteParentID)
	}

	value := strings.TrimSpace(existing.ValueString())
	if value == "" {
		return types.StringValue(remoteParentID)
	}

	resolvedParentID, err := resolveFolderReferenceValue(folders, value)
	if err == nil && resolvedParentID == remoteParentID {
		return existing
	}

	return types.StringValue(remoteParentID)
}
