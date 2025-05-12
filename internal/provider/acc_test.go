package provider_test

import (
	"terraform-provider-passbolt/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"passbolt": providerserver.NewProtocol6WithError(provider.New("test")()),
}
