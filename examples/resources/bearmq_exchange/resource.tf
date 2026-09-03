resource "bearmq_vhost" "orders" {
  name = "orders-prod"
}

# Primary topic exchange.
resource "bearmq_exchange" "orders" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders"
  type     = "TOPIC"
  durable  = true
}

# Direct dead-letter exchange with a delayed-delivery plugin arg.
resource "bearmq_exchange" "orders_dlx" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.dlx"
  type     = "DIRECT"
  durable  = true
  delayed  = true

  args = {
    "alternate-exchange" = "orders.unrouted"
  }
}
