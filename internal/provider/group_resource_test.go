package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPassboltGroup_fullLifecycle(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	suffix := testAccSuffix()
	groupName := testAccName("test-group", suffix)
	updatedGroupName := testAccName("renamed-group", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateGroup(baseURL, privateKey, passphrase, managerID, groupName),
			testStepNoDriftGroup(baseURL, privateKey, passphrase, managerID, groupName),
			testStepUpdateGroup(baseURL, privateKey, passphrase, managerID, updatedGroupName),
		},
	})
}

func testStepCreateGroup(baseURL, privateKey, passphrase, managerID, groupName string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
		),
	}
}

func testStepNoDriftGroup(baseURL, privateKey, passphrase, managerID, groupName string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
		),
	}
}

func testStepUpdateGroup(baseURL, privateKey, passphrase, managerID, groupName string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
		),
	}
}

func testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName string) string {
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
`, baseURL, privateKey, passphrase, groupName, managerID)
}
