package provider

import (
	"fmt"
	"os"
	"regexp"
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

const examplesDir = "../../examples/resources"

// loadExample reads an example .tf file and substitutes var.xxx references
// with the provided replacements. Variable declaration blocks are stripped
// so the resulting config is self-contained.
func loadExample(t *testing.T, resourceName string, vars map[string]string) string {
	t.Helper()
	path := fmt.Sprintf("%s/%s/resource.tf", examplesDir, resourceName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read example file %s: %s", path, err)
	}
	result := string(content)
	for name, replacement := range vars {
		result = replaceAll(result, "var."+name, replacement)
	}
	result = removeVariableBlocks(result)
	return result
}

// replaceAll replaces all occurrences of old with new in s.
// Wrapper to keep loadExample readable.
func replaceAll(s, old, new string) string {
	return regexp.MustCompile(regexp.QuoteMeta(old)).ReplaceAllLiteralString(s, new)
}

// removeVariableBlocks strips variable "..." { ... } declarations from HCL.
var variableBlockRe = regexp.MustCompile(`(?ms)^variable\s+"[^"]+"\s*\{[^}]*\}\s*\n?`)

func removeVariableBlocks(s string) string {
	return variableBlockRe.ReplaceAllString(s, "")
}

// prerequisiteProject returns HCL for a project resource named "prereq".
func prerequisiteProject(name string) string {
	return fmt.Sprintf(`
resource "sanity_project" "prereq" {
  name = %q
}
`, name)
}

// prerequisiteProjectAndDataset returns HCL for a project + dataset named "prereq".
func prerequisiteProjectAndDataset(projectName, datasetName string) string {
	return fmt.Sprintf(`
resource "sanity_project" "prereq" {
  name = %q
}

resource "sanity_dataset" "prereq" {
  project  = sanity_project.prereq.id
  name     = %q
  acl_mode = "public"
}
`, projectName, datasetName)
}

// --- Helper Tests ---

func TestRemoveVariableBlocks(t *testing.T) {
	input := `resource "sanity_project" "main" {
  name = var.project_name
}

variable "project_name" {
  description = "The name"
  type        = string
}

variable "other" {
  type = string
}
`
	got := removeVariableBlocks(input)
	if regexp.MustCompile(`variable\s+"`).MatchString(got) {
		t.Errorf("variable blocks were not removed:\n%s", got)
	}
	if !regexp.MustCompile(`resource "sanity_project"`).MatchString(got) {
		t.Errorf("resource block was incorrectly removed:\n%s", got)
	}
}

func TestLoadExample(t *testing.T) {
	config := loadExample(t, "sanity_project", map[string]string{
		"project_name": `"my-test"`,
	})
	if regexp.MustCompile(`var\.project_name`).MatchString(config) {
		t.Error("var.project_name was not replaced")
	}
	if !regexp.MustCompile(`"my-test"`).MatchString(config) {
		t.Error("replacement value not found in output")
	}
	if regexp.MustCompile(`variable\s+"`).MatchString(config) {
		t.Error("variable blocks were not stripped")
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
				Config: loadExample(t, "sanity_project", map[string]string{
					"project_name": fmt.Sprintf("%q", rName),
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_project.main", "id"),
					resource.TestCheckResourceAttr("sanity_project.main", "name", rName),
					resource.TestCheckResourceAttr("sanity_project.main", "color", "#0000ff"),
				),
			},
			// Import
			{
				ResourceName:      "sanity_project.main",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// --- Dataset ---

func TestAccDataset_basic(t *testing.T) {
	rProjectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	rDatasetName := fmt.Sprintf("tfacc%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProject(rProjectName) +
					loadExample(t, "sanity_dataset", map[string]string{
						"project_id":   "sanity_project.prereq.id",
						"dataset_name": fmt.Sprintf("%q", rDatasetName),
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sanity_dataset.main", "name", rDatasetName),
					resource.TestCheckResourceAttr("sanity_dataset.main", "acl_mode", "public"),
				),
			},
			// Import
			{
				ResourceName:      "sanity_dataset.main",
				ImportState:       true,
				ImportStateIdFunc: testAccDatasetImportID("sanity_project.prereq", "sanity_dataset.main"),
				ImportStateVerify: true,
			},
		},
	})
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
	rProjectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProject(rProjectName) +
					loadExample(t, "sanity_cors_origin", map[string]string{
						"project_id": "sanity_project.prereq.id",
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_cors_origin.main", "id"),
					resource.TestCheckResourceAttr("sanity_cors_origin.main", "origin", "https://example.com"),
					resource.TestCheckResourceAttr("sanity_cors_origin.main", "allow_credentials", "true"),
				),
			},
		},
	})
}

// --- Webhook ---

func TestAccWebhook_basic(t *testing.T) {
	rProjectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	rDatasetName := fmt.Sprintf("tfacc%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProjectAndDataset(rProjectName, rDatasetName) +
					loadExample(t, "sanity_webhook", map[string]string{
						"project_id":     "sanity_project.prereq.id",
						"dataset_name":   fmt.Sprintf("%q", rDatasetName),
						"webhook_secret": `"test-secret-123"`,
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_webhook.main", "id"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "name", "Content Updates Webhook"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "url", "https://api.example.com/webhooks/sanity"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "http_method", "POST"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "filter", "_type == 'post'"),
				),
			},
			// Import
			{
				ResourceName:            "sanity_webhook.main",
				ImportState:             true,
				ImportStateIdFunc:       testAccWebhookImportID("sanity_project.prereq", "sanity_webhook.main"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
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
	rProjectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	rDatasetName := fmt.Sprintf("tfacc%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProjectAndDataset(rProjectName, rDatasetName) +
					loadExample(t, "sanity_schema_type", map[string]string{
						"project_id":   "sanity_project.prereq.id",
						"dataset_name": fmt.Sprintf("%q", rDatasetName),
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify the article type
					resource.TestCheckResourceAttrSet("sanity_schema_type.article", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "name", "article"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "type", "document"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "title", "Article"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "field.#", "6"),
					// Verify the author type
					resource.TestCheckResourceAttrSet("sanity_schema_type.author", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.author", "name", "author"),
					resource.TestCheckResourceAttr("sanity_schema_type.author", "type", "document"),
					// Verify the category type
					resource.TestCheckResourceAttrSet("sanity_schema_type.category", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.category", "name", "category"),
					resource.TestCheckResourceAttr("sanity_schema_type.category", "type", "document"),
				),
			},
		},
	})
}

// --- Project Token ---

func TestAccProjectToken_basic(t *testing.T) {
	rProjectName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProject(rProjectName) +
					loadExample(t, "sanity_project_token", map[string]string{
						"project_id": "sanity_project.prereq.id",
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_project_token.main", "id"),
					resource.TestCheckResourceAttr("sanity_project_token.main", "label", "Deployer token"),
					resource.TestCheckResourceAttrSet("sanity_project_token.main", "key"),
				),
			},
		},
	})
}
