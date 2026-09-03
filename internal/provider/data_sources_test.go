package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSources_lookupByName(t *testing.T) {
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
  name = "acc-ds"
}

resource "bearmq_exchange" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders"
  type     = "TOPIC"
}

resource "bearmq_queue" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders.paid"
}

data "bearmq_vhost" "by_name" {
  name = bearmq_vhost.test.name
}

data "bearmq_exchange" "by_name" {
  vhost_id = bearmq_vhost.test.id
  name     = bearmq_exchange.test.name
}

data "bearmq_queue" "by_name" {
  vhost_id = bearmq_vhost.test.id
  name     = bearmq_queue.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.bearmq_vhost.by_name", "id", "bearmq_vhost.test", "id"),
					resource.TestCheckResourceAttr("data.bearmq_vhost.by_name", "status", "ACTIVE"),
					resource.TestCheckResourceAttrPair(
						"data.bearmq_exchange.by_name", "id", "bearmq_exchange.test", "id"),
					resource.TestCheckResourceAttr("data.bearmq_exchange.by_name", "type", "TOPIC"),
					resource.TestCheckResourceAttrPair(
						"data.bearmq_queue.by_name", "id", "bearmq_queue.test", "id"),
					resource.TestCheckResourceAttr("data.bearmq_queue.by_name", "status", "ACTIVE"),
				),
			},
		},
	})
}

func TestAccDataSource_vhostByID(t *testing.T) {
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
  name = "acc-ds-id"
}

data "bearmq_vhost" "by_id" {
  id = bearmq_vhost.test.id
}
`,
				Check: resource.TestCheckResourceAttr("data.bearmq_vhost.by_id", "name", "acc-ds-id"),
			},
		},
	})
}
