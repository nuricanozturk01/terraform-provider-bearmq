package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const bindingTopology = `
resource "bearmq_vhost" "test" {
  name = "acc-bind"
}

resource "bearmq_exchange" "src" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders"
  type     = "TOPIC"
}

resource "bearmq_queue" "dst" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders.paid"
}
`

func TestAccBindingResource_lifecycle(t *testing.T) {
	acceptancePreCheck(t)
	isolateEnv(t)
	srv, _ := newFakeBroker(t)
	p := providerBlock(srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: p + bindingTopology + `
resource "bearmq_binding" "test" {
  vhost_id         = bearmq_vhost.test.id
  source           = bearmq_exchange.src.name
  destination      = bearmq_queue.dst.name
  destination_type = "QUEUE"
  routing_key      = "order.paid"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bearmq_binding.test", "source", "orders"),
					resource.TestCheckResourceAttr("bearmq_binding.test", "destination", "orders.paid"),
					resource.TestCheckResourceAttr("bearmq_binding.test", "routing_key", "order.paid"),
					resource.TestCheckResourceAttr("bearmq_binding.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("bearmq_binding.test", "id"),
				),
			},
			{
				ResourceName:            "bearmq_binding.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateIdFunc:       compositeImportID("bearmq_binding.test"),
				ImportStateVerifyIgnore: []string{"destination_type", "arguments"},
			},
		},
	})
}

func TestAccBindingResource_unknownSourceFails(t *testing.T) {
	acceptancePreCheck(t)
	isolateEnv(t)
	srv, _ := newFakeBroker(t)
	p := providerBlock(srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: p + bindingTopology + `
resource "bearmq_binding" "test" {
  vhost_id         = bearmq_vhost.test.id
  source           = "does-not-exist"
  destination      = bearmq_queue.dst.name
  destination_type = "QUEUE"
}
`,
				ExpectError: regexp.MustCompile(`(?i)source exchange not found`),
			},
		},
	})
}
