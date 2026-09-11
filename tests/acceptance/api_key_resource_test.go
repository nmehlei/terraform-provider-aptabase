package acceptance_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nmehlei/terraform-provider-aptabase/tests/acceptance"
)

func TestAccApiKeyResource_Create(t *testing.T) {
	acceptance.RequireTFAcc(t)
	endpoint, token := acceptance.SharedBootstrap(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(endpoint, token) + `
resource "aptabase_api_key" "test" {
  name = "ci-key"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("aptabase_api_key.test", "name", "ci-key"),
					resource.TestCheckResourceAttrSet("aptabase_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("aptabase_api_key.test", "key_prefix"),
				),
			},
		},
	})
}
