# Manage individual content types as separate Terraform resources.
# Each resource represents one document or object type in a Sanity workspace schema.
# The provider handles read-modify-write merging automatically, so you can
# organise your schema across multiple .tf files (e.g. one file per content type).

# --- Article type ---

resource "sanity_schema_type" "article" {
  project_id     = sanity_project.blog.id
  dataset        = "production"
  workspace_name = "default"
  version        = "2025-05-01"

  name  = "article"
  type  = "document"
  title = "Article"

  field {
    name  = "title"
    type  = "string"
    title = "Title"
  }

  field {
    name  = "slug"
    type  = "slug"
    title = "Slug"
    options = jsonencode({
      source = "title"
    })
  }

  field {
    name  = "author"
    type  = "reference"
    title = "Author"
    to = jsonencode([
      { type = "author" }
    ])
  }

  field {
    name  = "publishedAt"
    type  = "datetime"
    title = "Published at"
  }

  field {
    name        = "mainImage"
    type        = "image"
    title       = "Main Image"
    description = "The hero image for this article."

    # Nested fields on the image asset (e.g. alt text)
    field {
      name  = "alt"
      type  = "string"
      title = "Alternative text"
    }
  }

  field {
    name  = "body"
    type  = "array"
    title = "Body"
    of = jsonencode([
      { type = "block" },
      { type = "image" }
    ])
  }
}

# --- Author type ---

resource "sanity_schema_type" "author" {
  project_id     = sanity_project.blog.id
  dataset        = "production"
  workspace_name = "default"
  version        = "2025-05-01"

  name  = "author"
  type  = "document"
  title = "Author"

  field {
    name  = "name"
    type  = "string"
    title = "Name"
  }

  field {
    name  = "bio"
    type  = "text"
    title = "Biography"
  }

  field {
    name  = "avatar"
    type  = "image"
    title = "Avatar"

    field {
      name  = "alt"
      type  = "string"
      title = "Alternative text"
    }
  }
}

# --- Category type ---

resource "sanity_schema_type" "category" {
  project_id     = sanity_project.blog.id
  dataset        = "production"
  workspace_name = "default"
  version        = "2025-05-01"

  name  = "category"
  type  = "document"
  title = "Category"

  field {
    name  = "title"
    type  = "string"
    title = "Title"
  }

  field {
    name  = "slug"
    type  = "slug"
    title = "Slug"
    options = jsonencode({
      source = "title"
    })
  }

  field {
    name  = "description"
    type  = "text"
    title = "Description"
  }
}
