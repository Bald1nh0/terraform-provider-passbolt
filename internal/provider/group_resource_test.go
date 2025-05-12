package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPassboltGroup_fullLifecycle(t *testing.T) {
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
			testStepCreateGroup(baseURL, privateKey, passphrase, managerID),
			testStepNoDriftGroup(baseURL, privateKey, passphrase, managerID),
			testStepUpdateGroup(baseURL, privateKey, passphrase, managerID),
		},
	})
}

func testStepCreateGroup(baseURL, privateKey, passphrase, managerID string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, "test-group"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", "test-group"),
		),
	}
}

func testStepNoDriftGroup(baseURL, privateKey, passphrase, managerID string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, "test-group"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", "test-group"),
		),
	}
}

func testStepUpdateGroup(baseURL, privateKey, passphrase, managerID string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, "renamed-group"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", "renamed-group"),
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
