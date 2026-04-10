//nolint:testpackage // Tests need access to unexported folder resolution helpers.
package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
)

func TestResolveFolderReferenceValue(t *testing.T) {
	t.Parallel()

	for _, tc := range folderReferenceTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualID, err := resolveFolderReferenceValue(testFolders(), tc.value)
			if tc.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.expectedErr)
				}
				if !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("expected error to contain %q, got %q", tc.expectedErr, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if actualID != tc.expectedID {
				t.Fatalf("expected folder id %q, got %q", tc.expectedID, actualID)
			}
		})
	}
}

func folderReferenceTestCases() []struct {
	name        string
	value       string
	expectedID  string
	expectedErr string
} {
	return []struct {
		name        string
		value       string
		expectedID  string
		expectedErr string
	}{
		{
			name:       "match by id",
			value:      "application-a-prod",
			expectedID: "application-a-prod",
		},
		{
			name:       "match by unique name",
			value:      "application_A",
			expectedID: "application-a",
		},
		{
			name:       "match by absolute path",
			value:      "/application_A/prod",
			expectedID: "application-a-prod",
		},
		{
			name:       "normalize trailing slash in path",
			value:      "/application_A/prod/",
			expectedID: "application-a-prod",
		},
		{
			name:        "reject whitespace-only reference",
			value:       "   ",
			expectedErr: "folder reference cannot be only whitespace",
		},
		{
			name:        "reject ambiguous name",
			value:       "dev",
			expectedErr: `folder name "dev" is ambiguous`,
		},
		{
			name:        "reject missing path",
			value:       "/application_A/stage",
			expectedErr: `folder with path "/application_A/stage" not found`,
		},
		{
			name:        "reject path with dot segments",
			value:       "/application_A/prod/../dev",
			expectedErr: `must not contain '.' or '..' segments`,
		},
	}
}

func TestBuildFolderPathIndex(t *testing.T) {
	t.Parallel()

	pathsByID, err := buildFolderPathIndex(testFolders())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if pathsByID["application-a-prod-sub-folder-3"] != "/application_A/prod/sub_folder_3" {
		t.Fatalf("expected nested path to be resolved, got %q", pathsByID["application-a-prod-sub-folder-3"])
	}
	if pathsByID["application-b-dev"] != "/application_B/dev" {
		t.Fatalf("expected sibling path to be resolved, got %q", pathsByID["application-b-dev"])
	}
}

func TestReconcileFolderParentReference(t *testing.T) {
	t.Parallel()

	folders := testFolders()

	kept := reconcileFolderParentReference(
		types.StringValue("/application_A/prod"),
		"application-a-prod",
		folders,
	)
	if kept.ValueString() != "/application_A/prod" {
		t.Fatalf("expected configured path to be preserved, got %q", kept.ValueString())
	}

	replaced := reconcileFolderParentReference(
		types.StringValue("dev"),
		"application-b-dev",
		folders,
	)
	if replaced.ValueString() != "application-b-dev" {
		t.Fatalf("expected ambiguous reference to be replaced with parent id, got %q", replaced.ValueString())
	}
}

func testFolders() []api.Folder {
	return []api.Folder{
		{ID: "application-a", Name: "application_A"},
		{ID: "application-a-dev", Name: "dev", FolderParentID: "application-a"},
		{ID: "application-a-prod", Name: "prod", FolderParentID: "application-a"},
		{ID: "application-a-prod-sub-folder-3", Name: "sub_folder_3", FolderParentID: "application-a-prod"},
		{ID: "application-b", Name: "application_B"},
		{ID: "application-b-dev", Name: "dev", FolderParentID: "application-b"},
	}
}
