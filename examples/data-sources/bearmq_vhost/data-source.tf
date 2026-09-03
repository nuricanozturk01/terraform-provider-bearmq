# Look up a virtual host by name (or set id instead — exactly one is required).
data "bearmq_vhost" "orders" {
  name = "orders-prod"
}

output "orders_vhost_id" {
  value = data.bearmq_vhost.orders.id
}
