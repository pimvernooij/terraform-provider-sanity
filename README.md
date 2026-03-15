# Terraform Provider for Sanity

The Terraform Sanity provider allows you to manage your
[Sanity](https://www.sanity.io/) projects with infrastructure-as-code
principles. It supports managing projects, datasets, CORS origins, project
tokens, and webhooks.

This provider is a fork of
[plain-insure/terraform-provider-sanity](https://github.com/plain-insure/terraform-provider-sanity),
which itself is a fork of the original
[labd/sanity](https://github.com/tessellator/terraform-provider-sanity) provider.

## Quick start

[Read the documentation](https://registry.terraform.io/providers/labd/sanity/latest/docs)
and check out the [examples](examples/).

## Usage

The provider is distributed via the Terraform registry. To use it you need to configure
the [`required_providers`](https://www.terraform.io/language/providers/requirements#requiring-providers) block:

```hcl
terraform {
  required_providers {
    sanity = {
      source = "labd/sanity"

      # It's recommended to pin the version, e.g.:
      # version = "~> 0.2.0"
    }
  }
}
```

Configure the provider with your Sanity API token. The token can be provided
via the `token` attribute or the `SANITY_TOKEN` environment variable:

```hcl
provider "sanity" {
  token = var.sanity_auth_token
}
```

### Example: creating a project with a dataset and webhook

```hcl
resource "sanity_project" "blog" {
  name        = "My blog"
  color       = "#0000ff"
  studio_host = "my-cool-blog"

  lifecycle {
    prevent_destroy = true
  }
}

resource "sanity_dataset" "production" {
  project  = sanity_project.blog.id
  name     = "production"
  acl_mode = "public"

  lifecycle {
    prevent_destroy = true
  }
}

resource "sanity_cors_origin" "frontend" {
  project           = sanity_project.blog.id
  origin            = "https://example.com"
  allow_credentials = true
}

resource "sanity_webhook" "deploy" {
  project_id = sanity_project.blog.id
  name       = "Deploy webhook"
  dataset    = sanity_dataset.production.name
  url        = "https://api.example.com/deploy"
}
```

## Resources

- `sanity_project` - Create and manage Sanity projects
- `sanity_dataset` - Manage datasets within a project
- `sanity_cors_origin` - Configure CORS origins for API access
- `sanity_project_token` - Create API tokens with specific roles
- `sanity_webhook` - Set up webhooks for content change notifications
- `sanity_schema` - Deploy and manage content schemas for a workspace (full JSON)
- `sanity_schema_type` - Manage individual content types with native HCL field blocks
- `sanity_studio_deployment` - Deploy a pre-built Sanity Studio bundle to Sanity hosting

## Data Sources

- `sanity_project` - Look up an existing Sanity project by ID
- `sanity_schema` - Read an existing deployed content schema

## Contributing

### Requirements

- [Go](https://golang.org/doc/install) >= 1.22
- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Task](https://taskfile.dev/) (optional, for development commands)

### Building the provider

```sh
git clone git@github.com:labd/terraform-provider-sanity.git
cd terraform-provider-sanity
task build-local
```

### Development commands

```sh
task build-local   # Build and install locally for manual testing
task test          # Run unit tests
task testacc       # Run acceptance tests (requires SANITY_TOKEN)
task lint          # Run golangci-lint
task format        # Format Go and Terraform files
task docs          # Generate provider documentation
task coverage      # Run tests with coverage report
```

### Running acceptance tests

Acceptance tests run against a real Sanity environment:

```sh
SANITY_TOKEN=<your-token> task testacc
```

### Running locally

To test a locally built provider, `task build-local` installs it to
`~/.terraform.d/plugins/`. Alternatively, create a `~/.terraformrc` with:

```hcl
provider_installation {
  dev_overrides {
    "labd/sanity" = "/path/to/terraform-provider-sanity"
  }

  direct {}
}
```

### Debugging

```sh
TF_LOG=DEBUG terraform plan   # Enable debug output
go run . -debug               # Run in debugger mode (for Delve)
```

## Authors

Originally developed by [tessellator](https://github.com/tessellator),
forked by [Plain](https://github.com/plain-insure),
maintained by [Lab Digital](https://github.com/labd).
