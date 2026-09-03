terraform {
  required_providers {
    bearmq = {
      source  = "nuricanozturk01/bearmq"
      version = "~> 0.1"
    }
  }
}

# Credentials and endpoint are normally supplied via environment variables:
#   BEARMQ_ENDPOINT  (default http://localhost:3333)
#   BEARMQ_API_KEY   (Settings -> Messaging API key, sent as X-API-KEY)
# or BEARMQ_TOKEN    (a JWT bearer token)
provider "bearmq" {
  endpoint = "http://localhost:3333"
  # api_key = var.bearmq_api_key
}
