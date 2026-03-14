# Read the default workspace schema
data "sanity_schema" "default" {
  project_id     = "abc123"
  dataset        = "production"
  workspace_name = "default"
}

output "schema_version" {
  value = data.sanity_schema.default.version
}

output "schema_types" {
  value = data.sanity_schema.default.schema
}

# Read a tagged schema
data "sanity_schema" "next" {
  project_id     = "abc123"
  dataset        = "staging"
  workspace_name = "default"
  tag            = "next"
}
