package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccExchangeResource_lifecycle(t *testing.T) {
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
  name = "acc-ex"
}

resource "bearmq_exchange" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders"
  type     = "topic"
  durable  = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bearmq_exchange.test", "name", "orders"),
					resource.TestCheckResourceAttr("bearmq_exchange.test", "type", "topic"),
					resource.TestCheckResourceAttr("bearmq_exchange.test", "durable", "true"),
					resource.TestCheckResourceAttr("bearmq_exchange.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("bearmq_exchange.test", "actual_name"),
				),
			},
			{
				ResourceName:      "bearmq_exchange.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: compositeImportID("bearmq_exchange.test"),
				// type/delayed/args are not refreshed from the server; import
				// starts them empty which is expected.
				ImportStateVerifyIgnore: []string{"type", "delayed", "args"},
			},
		},
	})
}

func TestAccExchangeResource_rejectsBadType(t *testing.T) {
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
  name = "acc-ex-bad"
}

resource "bearmq_exchange" "test" {
  vhost_id = bearmq_vhost.test.id
  name     = "orders"
  type     = "banana"
}
`,
				ExpectError: regexp.MustCompile(`(?i)value must be one of`),
			},
		},
	})
}
