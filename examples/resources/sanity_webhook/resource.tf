resource "sanity_webhook" "main" {
  project_id     = var.project_id
  name           = "Content Updates Webhook"
  dataset        = var.dataset_name
  url            = "https://example.com/webhooks/sanity"
  http_method    = "POST"
  include_drafts = false
  filter         = "_type == 'post'"
  secret         = var.webhook_secret

  headers = {
    "Content-Type" = "application/json"
  }
}

variable "project_id" {
  description = "The ID of the Sanity project"
  type        = string
}

variable "dataset_name" {
  description = "The dataset to listen to"
  type        = string
}

variable "webhook_secret" {
  description = "Secret for webhook signature verification"
  type        = string
  sensitive   = true
}
