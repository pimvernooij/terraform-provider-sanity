resource "sanity_cors_origin" "main" {
  project           = var.project_id
  origin            = "https://example.com"
  allow_credentials = true
}

variable "project_id" {
  description = "The ID of the Sanity project"
  type        = string
}
