terraform {
  required_providers {
    bearmq = {
      source  = "nuricanozturk01/bearmq"
      version = "~> 0.1"
    }
  }
}

# Credentials and endpoint are normally supplied via environment variables:
#   BEARMQ_ENDPOINT  the BearMQ instance URL
#   BEARMQ_API_KEY   (Settings -> Messaging API key, sent as X-API-KEY)
# or BEARMQ_TOKEN    (a JWT bearer token)
provider "bearmq" {
  endpoint = "https://api.bearmq.com"
  # api_key = var.bearmq_api_key
}
