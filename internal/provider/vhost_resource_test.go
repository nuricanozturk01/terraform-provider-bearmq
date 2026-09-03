package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVHostResource_lifecycle(t *testing.T) {
	acceptancePreCheck(t)
	isolateEnv(t)
	srv, _ := newFakeBroker(t)
	p := providerBlock(srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: p + `
resource "bearmq_vhost" "test" {
  name = "acc-vhost"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bearmq_vhost.test", "name", "acc-vhost"),
					resource.TestCheckResourceAttr("bearmq_vhost.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("bearmq_vhost.test", "id"),
					resource.TestCheckResourceAttrSet("bearmq_vhost.test", "username"),
					resource.TestCheckResourceAttrSet("bearmq_vhost.test", "password"),
					resource.TestCheckResourceAttrSet("bearmq_vhost.test", "url"),
				),
			},
			{
				// PAUSED is applied in place via the status endpoint (no replace).
				Config: p + `
resource "bearmq_vhost" "test" {
  name   = "acc-vhost"
  status = "PAUSED"
}
`,
				Check: resource.TestCheckResourceAttr("bearmq_vhost.test", "status", "PAUSED"),
			},
			{
				ResourceName:            "bearmq_vhost.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"}, // create-only field
			},
		},
	})
}

func TestAccVHostResource_disappears(t *testing.T) {
	acceptancePreCheck(t)
	isolateEnv(t)
	srv, fb := newFakeBroker(t)
	p := providerBlock(srv.URL)

	config := p + `
resource "bearmq_vhost" "gone" {
  name = "vanishing"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fb.mu.Lock()
					for id := range fb.vhosts {
						delete(fb.vhosts, id)
					}
					fb.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // 404 on read -> state removed -> recreate planned
			},
		},
	})
}
