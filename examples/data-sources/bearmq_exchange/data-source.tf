data "bearmq_vhost" "orders" {
  name = "orders-prod"
}

data "bearmq_exchange" "orders" {
  vhost_id = data.bearmq_vhost.orders.id
  name     = "orders"
}
