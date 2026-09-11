package acceptance_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nmehlei/terraform-provider-aptabase/tests/acceptance"
)

func providerConfig(endpoint, token string) string {
	return fmt.Sprintf(`
provider "aptabase" {
  endpoint = %q
  token    = %q
  insecure = true
}
`, endpoint, token)
}

func TestAccAppResource_CreateUpdateImport(t *testing.T) {
	acceptance.RequireTFAcc(t)
	endpoint, token := acceptance.SharedBootstrap(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(endpoint, token) + `
resource "aptabase_app" "test" {
  name = "Original Name"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("aptabase_app.test", "name", "Original Name"),
					resource.TestCheckResourceAttrSet("aptabase_app.test", "id"),
					resource.TestCheckResourceAttrSet("aptabase_app.test", "app_key"),
				),
			},
			{
				Config: providerConfig(endpoint, token) + `
resource "aptabase_app" "test" {
  name = "Renamed"
}
`,
				Check: resource.TestCheckResourceAttr("aptabase_app.test", "name", "Renamed"),
			},
			{
				Config:            providerConfig(endpoint, token) + `resource "aptabase_app" "test" { name = "Renamed" }`,
				ResourceName:      "aptabase_app.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
