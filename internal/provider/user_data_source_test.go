package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPassboltUserDataSource_lookup(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_TEST_USER_EMAIL")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	testEmail := os.Getenv("PASSBOLT_TEST_USER_EMAIL")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUserDataSourceConfig(baseURL, privateKey, passphrase, testEmail),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.passbolt_user.test", "username", testEmail),
					resource.TestCheckResourceAttrSet("data.passbolt_user.test", "id"),
					resource.TestCheckResourceAttrSet("data.passbolt_user.test", "role"),
				),
			},
		},
	})
}

func testUserDataSourceConfig(baseURL, privateKey, passphrase, email string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

data "passbolt_user" "test" {
  username = "%s"
}
`, baseURL, privateKey, passphrase, email)
}
