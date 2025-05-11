package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-passbolt/internal/provider"
)

var testAccProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"passbolt": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func TestAccPasswordResource_basic(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")

	if baseURL == "" || privateKey == "" || passphrase == "" {
		t.Skip("Acceptance tests skipped unless PASSBOLT_BASE_URL, PASSBOLT_PRIVATE_KEY, and PASSBOLT_PASSPHRASE are set")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreatePassword(baseURL, privateKey, passphrase),
			testStepUpdatePassword(baseURL, privateKey, passphrase),
		},
	})
}

func testStepCreatePassword(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			"acc-test", "user", "https://example.com", "super-secret"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", "acc-test"),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.com"),
		),
	}
}

func testStepUpdatePassword(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordConfig(baseURL,
			privateKey,
			passphrase,
			"acc-test-updated", "updated-user", "https://example.org", "new-secret"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", "acc-test-updated"),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "updated-user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.org"),
		),
	}
}

func testPasswordConfig(baseURL, privateKey, passphrase, name, username, uri, password string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_password" "example" {
  name     = "%s"
  username = "%s"
  uri      = "%s"
  password = "%s"
}
`, baseURL, privateKey, passphrase, name, username, uri, password)
}
