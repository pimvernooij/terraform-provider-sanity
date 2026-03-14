# Import an existing content type into Terraform state.
# Format: project_id/dataset/workspace_name/type_name
terraform import sanity_schema_type.article abc123/production/default/article

# With a tag:
# terraform import sanity_schema_type.article abc123/production/default/article/my-tag
