terraform {
  required_providers {
    sanity = {
      source = "labd/sanity"
    }
  }
  required_version = ">= 1.0"
}

provider "sanity" {
  token = "xxx"
}

resource "sanity_project" "my_blog" {
  name        = "My blog"
  color       = "#0000ff"
  studio_host = "my-cool-blog"

  lifecycle {
    prevent_destroy = true
  }
}

resource "sanity_cors_origin" "external" {
  project           = sanity_project.my_blog.id
  origin            = "https://example.com"
  allow_credentials = true
}

resource "sanity_dataset" "production" {
  project  = sanity_project.my_blog.id
  name     = "production"
  acl_mode = "public"

  lifecycle {
    prevent_destroy = true
  }
}

resource "sanity_project_token" "deployer" {
  project   = sanity_project.my_blog.id
  label     = "Deployer token"
  role_name = "deploy-studio"
}

resource "sanity_schema" "default" {
  project_id     = sanity_project.my_blog.id
  dataset        = sanity_dataset.production.name
  workspace_name = "default"
  version        = "2025-05-01"

  schema = jsonencode([
    {
      name  = "post"
      type  = "document"
      title = "Blog Post"
      fields = [
        { name = "title", type = "string", title = "Title" },
        { name = "slug", type = "slug", title = "Slug", options = { source = "title" } },
        {
          name  = "author"
          type  = "reference"
          title = "Author"
          to    = [{ type = "author" }]
        },
        {
          name  = "body"
          type  = "array"
          title = "Body"
          of    = [{ type = "block" }]
        },
      ]
    },
    {
      name  = "author"
      type  = "document"
      title = "Author"
      fields = [
        { name = "name", type = "string", title = "Name" },
        { name = "bio", type = "text", title = "Bio" },
      ]
    }
  ])
}

data "sanity_schema" "default" {
  project_id     = sanity_project.my_blog.id
  dataset        = sanity_dataset.production.name
  workspace_name = "default"

  depends_on = [sanity_schema.default]
}

# Deploy the studio
#
# Build workflow (run before terraform apply):
#   cd studio && npm ci && npx sanity build
#   tar -czf ../studio.tar.gz -C dist .
#
resource "sanity_studio_deployment" "production" {
  project_id = sanity_project.my_blog.id
  hostname   = "my-cool-blog"

  bundle           = "${path.module}/studio.tar.gz"
  source_code_hash = filebase64sha256("${path.module}/studio.tar.gz")

  depends_on = [sanity_schema.default]
}

output "studio_url" {
  value = sanity_studio_deployment.production.url
}
