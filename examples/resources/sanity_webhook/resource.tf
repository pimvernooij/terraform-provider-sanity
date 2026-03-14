resource "sanity_webhook" "example" {
  project_id     = sanity_project.example.id
  name           = "Content Updates Webhook"
  dataset        = "production"
  url            = "https://api.example.com/webhooks/sanity"
  http_method    = "POST"
  include_drafts = false
  filter         = "_type == 'post'"

  headers = {
    "Authorization" = "Bearer ${var.api_token}"
    "Content-Type"  = "application/json"
  }

  secret = var.webhook_secret
}

variable "api_token" {
  description = "API token for webhook authentication"
  type        = string
  sensitive   = true
}

variable "webhook_secret" {
  description = "Secret for webhook signature verification"
  type        = string
  sensitive   = true
}