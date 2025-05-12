package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPasswordDataSource_basic(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")

	if baseURL == "" || privateKey == "" || passphrase == "" {
		t.Skip("Acceptance tests skipped unless PASSBOLT_BASE_URL, PASSBOLT_PRIVATE_KEY, and PASSBOLT_PASSPHRASE are set")
	}

	resourceName := "acc-ds-test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_password" "example" {
  name     = "%s"
  username = "ds-user"
  uri      = "https://ds.example.com"
  password = "ds-secret"
}

data "passbolt_password" "by_id" {
  id = passbolt_password.example.id
}
`, baseURL, privateKey, passphrase, resourceName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.passbolt_password.by_id", "name", resourceName),
					resource.TestCheckResourceAttr("data.passbolt_password.by_id", "username", "ds-user"),
				),
			},
		},
	})
}
