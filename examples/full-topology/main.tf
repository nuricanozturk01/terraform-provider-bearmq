terraform {
  required_providers {
    bearmq = {
      source  = "nuricanozturk01/bearmq"
      version = "~> 0.1"
    }
  }
}

provider "bearmq" {
  # endpoint / api_key come from BEARMQ_ENDPOINT / BEARMQ_API_KEY
}

# One isolated virtual host for this environment.
resource "bearmq_vhost" "orders" {
  name   = "orders-prod"
  status = "ACTIVE"
}

# A topic exchange plus a direct dead-letter exchange.
resource "bearmq_exchange" "orders" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders"
  type     = "TOPIC"
  durable  = true
}

resource "bearmq_exchange" "orders_dlx" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.dlx"
  type     = "DIRECT"
  durable  = true
}

# Work queue with a TTL that dead-letters into orders.dlx.
resource "bearmq_queue" "paid" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.paid"
  durable  = true

  arguments = {
    "x-message-ttl"             = "60000"
    "x-dead-letter-exchange"    = "orders.dlx"
    "x-dead-letter-routing-key" = "orders.paid.dead"
  }
}

resource "bearmq_queue" "paid_dead" {
  vhost_id = bearmq_vhost.orders.id
  name     = "orders.paid.dead"
  durable  = true
}

# Bindings: live traffic + dead-letter path.
resource "bearmq_binding" "paid" {
  vhost_id         = bearmq_vhost.orders.id
  source           = bearmq_exchange.orders.name
  destination      = bearmq_queue.paid.name
  destination_type = "QUEUE"
  routing_key      = "order.paid"
}

resource "bearmq_binding" "paid_dead" {
  vhost_id         = bearmq_vhost.orders.id
  source           = bearmq_exchange.orders_dlx.name
  destination      = bearmq_queue.paid_dead.name
  destination_type = "QUEUE"
  routing_key      = "orders.paid.dead"
}

output "amqp_url" {
  value     = bearmq_vhost.orders.url
  sensitive = true
}

output "vhost_username" {
  value = bearmq_vhost.orders.username
}
