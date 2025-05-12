package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPassboltUser_fullLifecycle(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")

	if baseURL == "" || privateKey == "" || passphrase == "" {
		t.Skip("Set PASSBOLT_BASE_URL, PRIVATE_KEY and PASSPHRASE to run acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateUser(baseURL, privateKey, passphrase),
			testStepNoDriftUser(baseURL, privateKey, passphrase),
			testStepUpdateUser(baseURL, privateKey, passphrase),
		},
	})
}

func testStepCreateUser(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testUserConfig(baseURL, privateKey, passphrase,
			"acc.user@example.com", "Terraform", "User", "user"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_user.test", "username", "acc.user@example.com"),
			resource.TestCheckResourceAttr("passbolt_user.test", "first_name", "Terraform"),
			resource.TestCheckResourceAttr("passbolt_user.test", "last_name", "User"),
			resource.TestCheckResourceAttr("passbolt_user.test", "role", "user"),
		),
	}
}

func testStepNoDriftUser(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testUserConfig(baseURL, privateKey, passphrase,
			"acc.user@example.com", "Terraform", "User", "user"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_user.test", "role", "user"),
		),
	}
}

func testStepUpdateUser(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testUserConfig(baseURL, privateKey, passphrase,
			"acc.user@example.com", "Updated", "User", "admin"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_user.test", "first_name", "Updated"),
			resource.TestCheckResourceAttr("passbolt_user.test", "last_name", "User"),
			resource.TestCheckResourceAttr("passbolt_user.test", "role", "admin"),
		),
	}
}

func testUserConfig(baseURL, privateKey, passphrase, email, first, last, role string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_user" "test" {
  username   = "%s"
  first_name = "%s"
  last_name  = "%s"
  role       = "%s"
}
`, baseURL, privateKey, passphrase, email, first, last, role)
}
