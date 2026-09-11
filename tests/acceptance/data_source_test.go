package acceptance_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nmehlei/terraform-provider-aptabase/tests/acceptance"
)

func TestAccDataSources_AppAndApps(t *testing.T) {
	acceptance.RequireTFAcc(t)
	endpoint, token := acceptance.SharedBootstrap(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(endpoint, token) + `
resource "aptabase_app" "test" {
  name = "Data Source Target"
}

data "aptabase_app" "by_id" {
  id = aptabase_app.test.id
}

data "aptabase_apps" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aptabase_app.by_id", "name", "Data Source Target"),
					resource.TestCheckResourceAttrSet("data.aptabase_apps.all", "apps.#"),
				),
			},
		},
	})
}
