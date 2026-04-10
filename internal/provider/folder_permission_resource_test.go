package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFolderPermissionResource_basic(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	suffix := testAccSuffix()
	groupName := testAccName("test-perm-group", suffix)
	folderName := testAccName("shared-folder", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreatePermissionWithGroup(baseURL, privateKey, passphrase, managerID, groupName, folderName, "read"),
			testStepNoDriftPermissionWithGroup(baseURL, privateKey, passphrase, managerID, groupName, folderName, "read"),
			testStepUpdatePermissionWithGroup(baseURL, privateKey, passphrase, managerID, groupName, folderName, "owner"),
		},
	})
}

func testStepCreatePermissionWithGroup(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	groupName,
	folderName,
	permission string) resource.TestStep {
	return resource.TestStep{
		Config: testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, groupName, folderName, permission),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_folder.shared", "name", folderName),
			resource.TestCheckResourceAttr("passbolt_folder_permission.perm", "permission", permission),
		),
	}
}

func testStepNoDriftPermissionWithGroup(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	groupName,
	folderName,
	permission string) resource.TestStep {
	return resource.TestStep{
		Config: testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, groupName, folderName, permission),
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
	groupName,
	folderName,
	permission string) resource.TestStep {
	return resource.TestStep{
		Config: testPermissionWithGroupConfig(baseURL, privateKey, passphrase, managerID, groupName, folderName, permission),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder_permission.perm", "permission", permission),
		),
	}
}

func testPermissionWithGroupConfig(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	groupName,
	folderName,
	permission string,
) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "test" {
  name     = "%s"
  managers = ["%s"]
}

resource "passbolt_folder" "shared" {
  name = "%s"
}

resource "passbolt_folder_permission" "perm" {
  folder_id  = passbolt_folder.shared.id
  group_name = passbolt_group.test.name
  permission = "%s"
}
`, baseURL, privateKey, passphrase, groupName, managerID, folderName, permission)
}
