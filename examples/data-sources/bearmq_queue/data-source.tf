data "bearmq_vhost" "orders" {
  name = "orders-prod"
}

data "bearmq_queue" "paid" {
  vhost_id = data.bearmq_vhost.orders.id
  name     = "orders.paid"
}
