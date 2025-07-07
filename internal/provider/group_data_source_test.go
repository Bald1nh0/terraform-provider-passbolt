package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGroupDataSource_basic(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")

	if baseURL == "" || privateKey == "" || passphrase == "" || managerID == "" {
		t.Skip("Acceptance tests require PASSBOLT_BASE_URL, PRIVATE_KEY, PASSPHRASE and MANAGER_ID")
	}

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
  passphrase = "%s"
}

resource "passbolt_group" "test" {
  name     = "acc-group-test"
  managers = ["%s"]
}

data "passbolt_group" "by_name" {
  name = passbolt_group.test.name
}

output "group_id" {
  value = data.passbolt_group.by_name.id
}
`, baseURL, privateKey, passphrase, managerID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.passbolt_group.by_name", "id"),
					resource.TestCheckResourceAttr("data.passbolt_group.by_name", "name", "acc-group-test"),
				),
			},
		},
	})
}
