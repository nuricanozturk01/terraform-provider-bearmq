# An isolated AMQP namespace with its own generated credentials.
resource "bearmq_vhost" "orders" {
  name   = "orders-prod"
  status = "ACTIVE"
}

output "orders_amqp_url" {
  value     = bearmq_vhost.orders.url
  sensitive = true
}
