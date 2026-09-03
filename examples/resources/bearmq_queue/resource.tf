resource "bearmq_vhost" "orders" {
  name = "orders-prod"
}

# Dead-letter target.
resource "bearmq_queue" "paid_dead" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.paid.dead"
  durable  = true
}

# Work queue: 60s TTL, dead-letters into orders.dlx.
# Numeric-looking argument values are sent to the broker as JSON numbers.
resource "bearmq_queue" "paid" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.paid"
  durable  = true
  dlq_name = bearmq_queue.paid_dead.name

  arguments = {
    "x-message-ttl"             = "60000"
    "x-dead-letter-exchange"    = "orders.dlx"
    "x-dead-letter-routing-key" = "orders.paid.dead"
  }
}
