# Import a schema by project_id/dataset/workspace_name
terraform import sanity_schema.default abc123/production/default

# Import a tagged schema by project_id/dataset/workspace_name/tag
terraform import sanity_schema.default abc123/production/default/my-tag
