resource "sanity_dataset" "main" {
  project  = var.project_id
  name     = var.dataset_name
  acl_mode = "public"
}

variable "project_id" {
  description = "The ID of the Sanity project"
  type        = string
}

variable "dataset_name" {
  description = "The name of the dataset"
  type        = string
}
