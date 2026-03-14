# Deploy a pre-built studio bundle
#
# Build workflow:
#   cd studio
#   npm ci && npx sanity build
#   tar -czf ../studio.tar.gz -C dist .
#
resource "sanity_studio_deployment" "main" {
  project_id = sanity_project.blog.id
  hostname   = "my-company"

  bundle           = "${path.module}/studio.tar.gz"
  source_code_hash = filebase64sha256("${path.module}/studio.tar.gz")
}

# Deploy with auto-updates and explicit version
resource "sanity_studio_deployment" "staging" {
  project_id = sanity_project.blog.id
  hostname   = "my-company-staging"

  bundle           = "${path.module}/studio.tar.gz"
  source_code_hash = filebase64sha256("${path.module}/studio.tar.gz")

  auto_updates = true
  version      = "3.82.0"
}

# Use a data source to reference the deployed URL
output "studio_url" {
  value = sanity_studio_deployment.main.url
}
