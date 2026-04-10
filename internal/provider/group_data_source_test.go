package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGroupDataSource_basic(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	groupName := testAccName("acc-group-test", testAccSuffix())

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
  name     = "%s"
  managers = ["%s"]
}

data "passbolt_group" "by_name" {
  name = passbolt_group.test.name
}

output "group_id" {
  value = data.passbolt_group.by_name.id
}
`, baseURL, privateKey, passphrase, groupName, managerID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.passbolt_group.by_name", "id"),
					resource.TestCheckResourceAttr("data.passbolt_group.by_name", "name", groupName),
				),
			},
		},
	})
}
