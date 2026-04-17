package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

type groupUpdateRequest struct {
	ID           string                  `json:"id,omitempty"`
	Name         string                  `json:"name,omitempty"`
	GroupChanges []groupMembershipChange `json:"groups_users,omitempty"`
	Secrets      []api.Secret            `json:"secrets,omitempty"`
}

type groupMembershipChange struct {
	ID      string `json:"id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	IsAdmin *bool  `json:"is_admin,omitempty"`
	Delete  bool   `json:"delete,omitempty"`
}

func updateGroup(
	ctx context.Context,
	client *api.Client,
	groupID,
	name string,
	operations []helper.GroupMembershipOperation,
) error {
	currentMemberships, currentName, err := getCurrentGroupMemberships(ctx, client, groupID)
	if err != nil {
		return err
	}

	request, err := buildGroupUpdateRequest(groupID, name, currentName, currentMemberships, operations)
	if err != nil {
		return err
	}

	dryRun, err := updateGroupDryRun(ctx, client, groupID, request)
	if err != nil {
		return fmt.Errorf("update group dry-run: %w", err)
	}

	if err := appendMissingGroupSecrets(ctx, client, dryRun, &request); err != nil {
		return err
	}

	if err := saveGroupUpdate(ctx, client, groupID, request); err != nil {
		return err
	}

	return verifyGroupMembershipOperations(ctx, client, groupID, operations)
}

func buildGroupUpdateRequest(
	groupID,
	name,
	currentName string,
	currentMemberships []api.GroupMembership,
	operations []helper.GroupMembershipOperation,
) (groupUpdateRequest, error) {
	request := groupUpdateRequest{
		ID:           groupID,
		Name:         name,
		GroupChanges: []groupMembershipChange{},
		Secrets:      []api.Secret{},
	}
	if request.Name == "" {
		request.Name = currentName
	}

	for _, operation := range operations {
		change, err := buildGroupMembershipChange(currentMemberships, operation)
		if err != nil {
			return groupUpdateRequest{}, err
		}
		request.GroupChanges = append(request.GroupChanges, change)
	}

	return request, nil
}

func buildGroupMembershipChange(
	currentMemberships []api.GroupMembership,
	operation helper.GroupMembershipOperation,
) (groupMembershipChange, error) {
	membership, err := groupMembershipByUserID(currentMemberships, operation.UserID)
	if err != nil {
		if operation.Delete {
			return groupMembershipChange{}, fmt.Errorf("cannot delete user %v as it has no membership", operation.UserID)
		}

		return groupMembershipChange{
			UserID:  operation.UserID,
			IsAdmin: boolPtr(operation.IsGroupManager),
		}, nil
	}

	if !operation.Delete && membership.IsAdmin == operation.IsGroupManager {
		return groupMembershipChange{}, fmt.Errorf("membership for user %v already exists with same role", operation.UserID)
	}

	change := groupMembershipChange{
		ID:     membership.ID,
		Delete: operation.Delete,
	}
	if !operation.Delete {
		change.IsAdmin = boolPtr(operation.IsGroupManager)
	}

	return change, nil
}

func getCurrentGroupMemberships(
	ctx context.Context,
	client *api.Client,
	groupID string,
) ([]api.GroupMembership, string, error) {
	groups, err := client.GetGroups(ctx, &api.GetGroupsOptions{
		ContainGroupsUsers: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("getting groups: %w", err)
	}

	for _, group := range groups {
		if group.ID == groupID {
			return group.GroupUsers, group.Name, nil
		}
	}

	return nil, "", fmt.Errorf("cannot find group with ID %v", groupID)
}

func updateGroupDryRun(
	ctx context.Context,
	client *api.Client,
	groupID string,
	request groupUpdateRequest,
) (*api.UpdateGroupDryRunResult, error) {
	msg, err := client.DoCustomRequest(ctx, "PUT", "/groups/"+groupID+"/dry-run.json", "v2", request, nil)
	if err != nil {
		return nil, err
	}

	var result api.UpdateGroupDryRunResult
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func appendMissingGroupSecrets(
	ctx context.Context,
	client *api.Client,
	dryRun *api.UpdateGroupDryRunResult,
	request *groupUpdateRequest,
) error {
	if len(dryRun.DryRun.SecretsNeeded) == 0 {
		return nil
	}

	users, err := client.GetUsers(ctx, &api.GetUsersOptions{})
	if err != nil {
		return fmt.Errorf("getting users: %w", err)
	}

	secrets := flattenGroupSecrets(dryRun.DryRun.Secrets)
	decryptedSecretCache := map[string]string{}
	currentUserID := client.GetUserID()

	for _, container := range dryRun.DryRun.SecretsNeeded {
		missingSecret := container.Secret
		decryptedSecret, ok := decryptedSecretCache[missingSecret.ResourceID]
		if !ok {
			decryptedSecret, err := decryptedGroupSecretByResourceID(
				secrets,
				missingSecret.ResourceID,
				currentUserID,
				client.DecryptMessage,
			)
			if err != nil {
				return fmt.Errorf("decrypting secret: %w", err)
			}
			decryptedSecretCache[missingSecret.ResourceID] = decryptedSecret
		}

		publicKey, err := groupUserPublicKey(missingSecret.UserID, users)
		if err != nil {
			return fmt.Errorf("get public key for user: %w", err)
		}

		newSecretData, err := client.EncryptMessageWithPublicKey(publicKey, decryptedSecret)
		if err != nil {
			return fmt.Errorf("encrypting secret: %w", err)
		}

		request.Secrets = append(request.Secrets, api.Secret{
			UserID:     missingSecret.UserID,
			ResourceID: missingSecret.ResourceID,
			Data:       newSecretData,
		})
	}

	return nil
}

func saveGroupUpdate(
	ctx context.Context,
	client *api.Client,
	groupID string,
	request groupUpdateRequest,
) error {
	msg, err := client.DoCustomRequest(ctx, "PUT", "/groups/"+groupID+".json", "v2", request, nil)
	if err != nil {
		return fmt.Errorf("updating group: %w", err)
	}

	var group api.Group
	if err := json.Unmarshal(msg.Body, &group); err != nil {
		return err
	}

	return nil
}

func verifyGroupMembershipOperations(
	ctx context.Context,
	client *api.Client,
	groupID string,
	operations []helper.GroupMembershipOperation,
) error {
	if len(operations) == 0 {
		return nil
	}

	memberships, _, err := getCurrentGroupMemberships(ctx, client, groupID)
	if err != nil {
		return err
	}

	for _, operation := range operations {
		membership, err := groupMembershipByUserID(memberships, operation.UserID)
		if operation.Delete {
			if err == nil {
				return fmt.Errorf("group membership for user %v was not deleted", operation.UserID)
			}

			continue
		}

		if err != nil {
			return fmt.Errorf(
				"group membership for user %v was not applied; ensure the provider-authenticated user is a "+
					"manager of the group and the target user is active",
				operation.UserID,
			)
		}

		if membership.IsAdmin != operation.IsGroupManager {
			return fmt.Errorf("group membership role for user %v was not applied", operation.UserID)
		}
	}

	return nil
}

func flattenGroupSecrets(containers []api.GroupSecret) []api.Secret {
	secrets := []api.Secret{}
	for _, container := range containers {
		secrets = append(secrets, container.Secret...)
	}

	return secrets
}

func groupMembershipByUserID(
	memberships []api.GroupMembership,
	userID string,
) (*api.GroupMembership, error) {
	for _, membership := range memberships {
		if membership.UserID == userID {
			return &membership, nil
		}
	}

	return nil, fmt.Errorf("cannot find membership for user ID %v", userID)
}

func decryptedGroupSecretByResourceID(
	secrets []api.Secret,
	resourceID,
	currentUserID string,
	decrypt func(string) (string, error),
) (string, error) {
	candidates := groupSecretsByResourceID(secrets, resourceID)
	if len(candidates) == 0 {
		return "", fmt.Errorf("cannot find secret for resource ID %v", resourceID)
	}

	tryDecrypt := func(secret api.Secret) (string, bool) {
		decryptedSecret, err := decrypt(secret.Data)
		if err != nil {
			return "", false
		}

		return decryptedSecret, true
	}

	if currentUserID != "" {
		for _, secret := range candidates {
			if secret.UserID != currentUserID {
				continue
			}

			if decryptedSecret, ok := tryDecrypt(secret); ok {
				return decryptedSecret, nil
			}
		}
	}

	for _, secret := range candidates {
		if secret.UserID == currentUserID {
			continue
		}

		if decryptedSecret, ok := tryDecrypt(secret); ok {
			return decryptedSecret, nil
		}
	}

	return "", fmt.Errorf("cannot decrypt secret for resource ID %v", resourceID)
}

func groupSecretsByResourceID(secrets []api.Secret, resourceID string) []api.Secret {
	matches := make([]api.Secret, 0)
	for _, secret := range secrets {
		if secret.ResourceID == resourceID {
			matches = append(matches, secret)
		}
	}

	return matches
}

func groupUserPublicKey(userID string, users []api.User) (string, error) {
	for _, user := range users {
		if user.ID == userID && user.GPGKey != nil {
			return user.GPGKey.ArmoredKey, nil
		}
	}

	return "", fmt.Errorf("cannot find public key for user ID %v", userID)
}

func boolPtr(value bool) *bool {
	return &value
}
