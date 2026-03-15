resource "sanity_project" "main" {
  name  = var.project_name
  color = "#0000ff"
}

variable "project_name" {
  description = "The name of the Sanity project"
  type        = string
}
