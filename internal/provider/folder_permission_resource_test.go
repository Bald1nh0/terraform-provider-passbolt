package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFolderPermissionResource_basic(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")

	if baseURL == "" || privateKey == "" || passphrase == "" || managerID == "" {
		t.Skip("PASSBOLT_BASE_URL, PRIVATE_KEY, PASSPHRASE, and MANAGER_ID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreatePermissionWithGroup(baseURL, privateKey, passphrase, managerID, "read"),
			testStepNoDriftPermissionWithGroup(baseURL, privateKey, passphrase, managerID, "read"),
			testStepUpdatePermissionWithGroup(baseURL, privateKey, passphrase, managerID, "owner"),
		},
	})
}

func testStepCreatePermissionWithGroup(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	permission string) resource.TestStep {
	return resource.TestStep{
		Config: testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, permission),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", "test-perm-group"),
			resource.TestCheckResourceAttr("passbolt_folder.shared", "name", "shared-folder"),
			resource.TestCheckResourceAttr("passbolt_folder_permission.perm", "permission", permission),
		),
	}
}

func testStepNoDriftPermissionWithGroup(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	permission string) resource.TestStep {
	return resource.TestStep{
		Config: testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, permission),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder_permission.perm", "permission", permission),
		),
	}
}

func testStepUpdatePermissionWithGroup(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	permission string) resource.TestStep {
	return resource.TestStep{
		Config: testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, permission),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder_permission.perm", "permission", permission),
		),
	}
}

func testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, permission string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "test" {
  name     = "test-perm-group"
  managers = ["%s"]
}

resource "passbolt_folder" "shared" {
  name = "shared-folder"
}

resource "passbolt_folder_permission" "perm" {
  folder_id  = passbolt_folder.shared.id
  group_name = passbolt_group.test.name
  permission = "%s"
}
`, baseURL, privateKey, passphrase, managerID, permission)
}
