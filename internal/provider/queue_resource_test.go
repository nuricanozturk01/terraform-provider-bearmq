package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccQueueResource_lifecycle(t *testing.T) {
	acceptancePreCheck(t)
	isolateEnv(t)
	srv, _ := newFakeBroker(t)
	p := providerBlock(srv.URL)

	base := p + `
resource "bearmq_vhost" "test" {
  name = "acc-q"
}

resource "bearmq_queue" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders.paid"
  durable  = true

  arguments = {
    "x-message-ttl" = "60000"
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: base,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bearmq_queue.test", "name", "orders.paid"),
					resource.TestCheckResourceAttr("bearmq_queue.test", "durable", "true"),
					resource.TestCheckResourceAttr("bearmq_queue.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("bearmq_queue.test", "id"),
					resource.TestCheckResourceAttrSet("bearmq_queue.test", "actual_name"),
					resource.TestCheckResourceAttrPair(
						"bearmq_queue.test", "vhost_id", "bearmq_vhost.test", "id"),
				),
			},
			{
				ResourceName:            "bearmq_queue.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateIdFunc:       compositeImportID("bearmq_queue.test"),
				ImportStateVerifyIgnore: []string{"arguments"}, // not returned by the read endpoint
			},
			{
				// Changing an immutable attribute forces replacement.
				Config: p + `
resource "bearmq_vhost" "test" {
  name = "acc-q"
}

resource "bearmq_queue" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders.paid.v2"
  durable  = true

  arguments = {
    "x-message-ttl" = "60000"
  }
}
`,
				Check: resource.TestCheckResourceAttr("bearmq_queue.test", "name", "orders.paid.v2"),
			},
		},
	})
}

func TestAccQueueResource_recreatedWhenDeletedOutOfBand(t *testing.T) {
	acceptancePreCheck(t)
	isolateEnv(t)
	srv, fb := newFakeBroker(t)
	p := providerBlock(srv.URL)

	config := p + `
resource "bearmq_vhost" "test" {
  name = "acc-drift"
}

resource "bearmq_queue" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "ephemeral"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fb.mu.Lock()
					for _, qs := range fb.queues {
						for qid := range qs {
							delete(qs, qid)
						}
					}
					fb.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// compositeImportID builds the "<vhost_id>/<resource_id>" import identifier from
// prior state for a resource that carries both a vhost_id and an id attribute.
func compositeImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource %s not found in state", resourceName)
		}
		return rs.Primary.Attributes["vhost_id"] + "/" + rs.Primary.ID, nil
	}
}
