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
					resource.TestCheckResourceAttr("data.passbolt_user.test", "include_inactive", "false"),
					resource.TestCheckResourceAttr("data.passbolt_user.test", "active", "true"),
					resource.TestCheckResourceAttrSet("data.passbolt_user.test", "id"),
					resource.TestCheckResourceAttrSet("data.passbolt_user.test", "role"),
				),
			},
		},
	})
}

func TestAccPassboltUserDataSource_includeInactive(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	email := testAccEmail("inactive.datasource", testAccSuffix())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUserDataSourceIncludeInactiveConfig(baseURL, privateKey, passphrase, email),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.passbolt_user.test", "username", email),
					resource.TestCheckResourceAttr("data.passbolt_user.test", "include_inactive", "true"),
					resource.TestCheckResourceAttr("data.passbolt_user.test", "active", "false"),
					resource.TestCheckResourceAttrPair(
						"data.passbolt_user.test",
						"id",
						"passbolt_user.test",
						"id",
					),
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

func testUserDataSourceIncludeInactiveConfig(baseURL, privateKey, passphrase, email string) string {
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
  first_name = "Pending"
  last_name  = "Datasource"
  role       = "user"
}

data "passbolt_user" "test" {
  username         = passbolt_user.test.username
  include_inactive = true

  depends_on = [passbolt_user.test]
}
`, baseURL, privateKey, passphrase, email)
}
