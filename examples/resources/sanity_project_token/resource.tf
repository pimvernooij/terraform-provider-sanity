resource "sanity_project_token" "main" {
  project   = var.project_id
  label     = "Deployer token"
  role_name = "deploy-studio"
}

variable "project_id" {
  description = "The ID of the Sanity project"
  type        = string
}
