package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccProtoV6ProviderFactories returns provider factories for acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"sanity": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates required environment variables are set.
func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("SANITY_TOKEN"); v == "" {
		t.Fatal("SANITY_TOKEN must be set for acceptance tests")
	}
}

// --- Project ---

func TestAccProject_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_project.test", "id"),
					resource.TestCheckResourceAttr("sanity_project.test", "name", rName),
					resource.TestCheckResourceAttr("sanity_project.test", "color", "#ff0000"),
				),
			},
			// Import
			{
				ResourceName:      "sanity_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccProjectConfig(name string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name  = %q
  color = "#ff0000"
}
`, name)
}

// --- Dataset ---

func TestAccDataset_basic(t *testing.T) {
	rName := fmt.Sprintf("tfacc%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	projectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig(projectName, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sanity_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("sanity_dataset.test", "acl_mode", "public"),
				),
			},
			// Import
			{
				ResourceName:      "sanity_dataset.test",
				ImportState:       true,
				ImportStateIdFunc: testAccDatasetImportID("sanity_project.test", "sanity_dataset.test"),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatasetConfig(projectName, datasetName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_dataset" "test" {
  project  = sanity_project.test.id
  name     = %q
  acl_mode = "public"
}
`, projectName, datasetName)
}

func testAccDatasetImportID(projectRes, datasetRes string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		project, ok := s.RootModule().Resources[projectRes]
		if !ok {
			return "", fmt.Errorf("resource %s not found", projectRes)
		}
		dataset, ok := s.RootModule().Resources[datasetRes]
		if !ok {
			return "", fmt.Errorf("resource %s not found", datasetRes)
		}
		return fmt.Sprintf("%s/%s", project.Primary.ID, dataset.Primary.Attributes["name"]), nil
	}
}

// --- CORS Origin ---

func TestAccCORSOrigin_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCORSOriginConfig(projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_cors_origin.test", "id"),
					resource.TestCheckResourceAttr("sanity_cors_origin.test", "origin", "https://terraform-test.example.com"),
					resource.TestCheckResourceAttr("sanity_cors_origin.test", "allow_credentials", "true"),
				),
			},
		},
	})
}

func testAccCORSOriginConfig(projectName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_cors_origin" "test" {
  project           = sanity_project.test.id
  origin            = "https://terraform-test.example.com"
  allow_credentials = true
}
`, projectName)
}

// --- Webhook ---

func TestAccWebhook_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	datasetName := fmt.Sprintf("tfacc%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookConfig(projectName, datasetName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_webhook.test", "id"),
					resource.TestCheckResourceAttr("sanity_webhook.test", "name", "TF Acceptance Test"),
					resource.TestCheckResourceAttr("sanity_webhook.test", "url", "https://httpbin.org/post"),
					resource.TestCheckResourceAttr("sanity_webhook.test", "http_method", "POST"),
					resource.TestCheckResourceAttr("sanity_webhook.test", "filter", "_type == 'post'"),
				),
			},
			// Update name and URL
			{
				Config: testAccWebhookConfigUpdated(projectName, datasetName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sanity_webhook.test", "name", "TF Acceptance Test Updated"),
					resource.TestCheckResourceAttr("sanity_webhook.test", "url", "https://httpbin.org/put"),
				),
			},
			// Import
			{
				ResourceName:            "sanity_webhook.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccWebhookImportID("sanity_project.test", "sanity_webhook.test"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func testAccWebhookConfig(projectName, datasetName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_dataset" "test" {
  project  = sanity_project.test.id
  name     = %q
  acl_mode = "public"
}

resource "sanity_webhook" "test" {
  project_id = sanity_project.test.id
  name       = "TF Acceptance Test"
  dataset    = sanity_dataset.test.name
  url        = "https://httpbin.org/post"
  filter     = "_type == 'post'"
  secret     = "test-secret-123"
}
`, projectName, datasetName)
}

func testAccWebhookConfigUpdated(projectName, datasetName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_dataset" "test" {
  project  = sanity_project.test.id
  name     = %q
  acl_mode = "public"
}

resource "sanity_webhook" "test" {
  project_id = sanity_project.test.id
  name       = "TF Acceptance Test Updated"
  dataset    = sanity_dataset.test.name
  url        = "https://httpbin.org/put"
  filter     = "_type == 'post'"
  secret     = "test-secret-123"
}
`, projectName, datasetName)
}

func testAccWebhookImportID(projectRes, webhookRes string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		project, ok := s.RootModule().Resources[projectRes]
		if !ok {
			return "", fmt.Errorf("resource %s not found", projectRes)
		}
		webhook, ok := s.RootModule().Resources[webhookRes]
		if !ok {
			return "", fmt.Errorf("resource %s not found", webhookRes)
		}
		return fmt.Sprintf("%s/%s", project.Primary.ID, webhook.Primary.ID), nil
	}
}

// --- Schema Type ---

func TestAccSchemaType_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	datasetName := fmt.Sprintf("tfacc%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSchemaTypeConfig(projectName, datasetName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_schema_type.post", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.post", "name", "post"),
					resource.TestCheckResourceAttr("sanity_schema_type.post", "type", "document"),
					resource.TestCheckResourceAttr("sanity_schema_type.post", "title", "Blog Post"),
				),
			},
			// Update: add a field
			{
				Config: testAccSchemaTypeConfigUpdated(projectName, datasetName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sanity_schema_type.post", "title", "Blog Post"),
					resource.TestCheckResourceAttr("sanity_schema_type.post", "field.#", "3"),
				),
			},
		},
	})
}

func testAccSchemaTypeConfig(projectName, datasetName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_dataset" "test" {
  project  = sanity_project.test.id
  name     = %q
  acl_mode = "public"
}

resource "sanity_schema_type" "post" {
  project_id     = sanity_project.test.id
  dataset        = sanity_dataset.test.name
  workspace_name = "default"
  version        = "2025-05-01"

  name  = "post"
  type  = "document"
  title = "Blog Post"

  field {
    name  = "title"
    type  = "string"
    title = "Title"
  }

  field {
    name  = "slug"
    type  = "slug"
    title = "Slug"
    options = jsonencode({ source = "title" })
  }
}
`, projectName, datasetName)
}

func testAccSchemaTypeConfigUpdated(projectName, datasetName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_dataset" "test" {
  project  = sanity_project.test.id
  name     = %q
  acl_mode = "public"
}

resource "sanity_schema_type" "post" {
  project_id     = sanity_project.test.id
  dataset        = sanity_dataset.test.name
  workspace_name = "default"
  version        = "2025-05-01"

  name  = "post"
  type  = "document"
  title = "Blog Post"

  field {
    name  = "title"
    type  = "string"
    title = "Title"
  }

  field {
    name  = "slug"
    type  = "slug"
    title = "Slug"
    options = jsonencode({ source = "title" })
  }

  field {
    name  = "body"
    type  = "array"
    title = "Body"
    of    = jsonencode([{ type = "block" }])
  }
}
`, projectName, datasetName)
}

// --- Project Token ---

func TestAccProjectToken_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectTokenConfig(projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_project_token.test", "id"),
					resource.TestCheckResourceAttr("sanity_project_token.test", "label", "tf-acc-test"),
					resource.TestCheckResourceAttrSet("sanity_project_token.test", "key"),
				),
			},
		},
	})
}

func testAccProjectTokenConfig(projectName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "test" {
  name = %q
}

resource "sanity_project_token" "test" {
  project   = sanity_project.test.id
  label     = "tf-acc-test"
  role_name = "viewer"
}
`, projectName)
}
