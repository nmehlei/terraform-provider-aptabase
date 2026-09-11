package acceptance_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nmehlei/terraform-provider-aptabase/tests/acceptance"
)

func TestAccAppShareResource_CreateAndDestroy(t *testing.T) {
	acceptance.RequireTFAcc(t)
	endpoint, token := acceptance.Bootstrap(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(endpoint, token) + `
resource "aptabase_app" "test" {
  name = "Shared App"
}

resource "aptabase_app_share" "test" {
  app_id = aptabase_app.test.id
  email  = "friend@example.com"
}
`,
				Check: resource.TestCheckResourceAttr("aptabase_app_share.test", "email", "friend@example.com"),
			},
			{
				Config: providerConfig(endpoint, token) + `
resource "aptabase_app" "test" {
  name = "Shared App"
}
`,
				// Removing the aptabase_app_share.test block from config causes
				// Terraform to fully destroy it, not merely clear its "email"
				// attribute - so resource.TestCheckNoResourceAttr would fail with
				// "Not found" (it requires the resource to still be present in
				// state). Assert directly that it is gone from state instead.
				Check: func(s *terraform.State) error {
					if _, ok := s.RootModule().Resources["aptabase_app_share.test"]; ok {
						return fmt.Errorf("aptabase_app_share.test still present in state, expected it to be destroyed")
					}
					return nil
				},
			},
		},
	})
}
