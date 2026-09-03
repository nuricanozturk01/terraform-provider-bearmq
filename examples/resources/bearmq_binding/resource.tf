resource "bearmq_vhost" "orders" {
  name = "orders-prod"
}

resource "bearmq_exchange" "orders" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders"
  type     = "TOPIC"
}

resource "bearmq_queue" "paid" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.paid"
}

# Route "order.paid" messages from the exchange into the queue.
resource "bearmq_binding" "paid" {
  vhost_id         = bearmq_vhost.orders.id
  source           = bearmq_exchange.orders.name
  destination      = bearmq_queue.paid.name
  destination_type = "QUEUE"
  routing_key      = "order.paid"
}
