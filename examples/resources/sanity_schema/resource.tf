# Basic schema with a blog content model
resource "sanity_schema" "default" {
  project_id     = sanity_project.blog.id
  dataset        = "production"
  workspace_name = "default"
  version        = "2025-05-01"

  schema = jsonencode([
    {
      name  = "post"
      type  = "document"
      title = "Blog Post"
      fields = [
        { name = "title", type = "string", title = "Title" },
        {
          name  = "slug"
          type  = "slug"
          title = "Slug"
          options = {
            source    = "title"
            maxLength = 96
          }
        },
        {
          name  = "author"
          type  = "reference"
          title = "Author"
          to    = [{ type = "author" }]
        },
        {
          name  = "mainImage"
          type  = "image"
          title = "Main Image"
          options = {
            hotspot = true
          }
          fields = [
            { name = "alt", type = "string", title = "Alternative Text" }
          ]
        },
        {
          name  = "categories"
          type  = "array"
          title = "Categories"
          of    = [{ type = "reference", to = [{ type = "category" }] }]
        },
        {
          name  = "publishedAt"
          type  = "datetime"
          title = "Published At"
        },
        {
          name  = "body"
          type  = "array"
          title = "Body"
          of = [
            { type = "block" },
            { type = "image", options = { hotspot = true } },
            {
              name  = "code"
              type  = "object"
              title = "Code Block"
              fields = [
                { name = "language", type = "string", title = "Language" },
                { name = "code", type = "text", title = "Code" },
              ]
            }
          ]
        },
      ]
    },
    {
      name  = "author"
      type  = "document"
      title = "Author"
      fields = [
        { name = "name", type = "string", title = "Name" },
        { name = "slug", type = "slug", title = "Slug", options = { source = "name" } },
        { name = "image", type = "image", title = "Image", options = { hotspot = true } },
        {
          name  = "bio"
          type  = "array"
          title = "Bio"
          of    = [{ type = "block" }]
        },
      ]
    },
    {
      name  = "category"
      type  = "document"
      title = "Category"
      fields = [
        { name = "title", type = "string", title = "Title" },
        { name = "description", type = "text", title = "Description" },
      ]
    },
    {
      name  = "siteSettings"
      type  = "document"
      title = "Site Settings"
      fields = [
        { name = "title", type = "string", title = "Site Title" },
        { name = "description", type = "text", title = "Site Description" },
        {
          name  = "logo"
          type  = "image"
          title = "Logo"
        },
        {
          name  = "socialLinks"
          type  = "array"
          title = "Social Links"
          of = [{
            type  = "object"
            name  = "socialLink"
            title = "Social Link"
            fields = [
              { name = "platform", type = "string", title = "Platform" },
              { name = "url", type = "url", title = "URL" },
            ]
          }]
        },
      ]
    }
  ])
}

# Schema with a workspace title and tag
resource "sanity_schema" "staging" {
  project_id      = sanity_project.blog.id
  dataset         = "staging"
  workspace_name  = "default"
  workspace_title = "Staging Studio"
  version         = "2025-05-01"
  tag             = "next"

  schema = jsonencode([
    {
      name  = "post"
      type  = "document"
      title = "Blog Post"
      fields = [
        { name = "title", type = "string", title = "Title" },
        { name = "slug", type = "slug", title = "Slug", options = { source = "title" } },
        { name = "body", type = "array", title = "Body", of = [{ type = "block" }] },
      ]
    }
  ])
}

# Schema loaded from an external JSON file
resource "sanity_schema" "from_file" {
  project_id     = sanity_project.blog.id
  dataset        = "production"
  workspace_name = "default"
  version        = "2025-05-01"

  schema = file("${path.module}/schema.json")
}
