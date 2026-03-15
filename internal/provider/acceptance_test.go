package provider

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/oauth2"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"
)

const examplesDir = "../../examples/resources"

// providerFactoriesWithRecorder creates provider factories backed by a go-vcr
// recorder. In replay mode (default), cassettes are replayed without hitting
// the real API. In record mode (RECORD=true), real API calls are made and
// captured to cassettes.
func providerFactoriesWithRecorder(cassetteName string) (map[string]func() (tfprotov6.ProviderServer, error), func()) {
	mode := recorder.ModeReplayOnly
	if os.Getenv("RECORD") == "true" {
		mode = recorder.ModeRecordOnly
	}

	r, err := recorder.NewWithOptions(&recorder.Options{
		CassetteName:       fmt.Sprintf("testdata/cassettes/%s", cassetteName),
		Mode:               mode,
		SkipRequestLatency: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// When recording, set the oauth2 transport as real transport so requests
	// are authenticated. During replay this is unused.
	if mode == recorder.ModeRecordOnly {
		token := os.Getenv("SANITY_TOKEN")
		if token == "" {
			log.Fatal("SANITY_TOKEN must be set when RECORD=true")
		}
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		r.SetRealTransport(&oauth2.Transport{Source: ts})
	}

	// Strip sensitive and noisy headers from cassettes.
	r.AddHook(func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")
		for key := range i.Response.Headers {
			if key != "Content-Type" {
				delete(i.Response.Headers, key)
			}
		}
		return nil
	}, recorder.AfterCaptureHook)

	client := r.GetDefaultClient()

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"sanity": providerserver.NewProtocol6WithError(New("test", WithHTTPClient(client))()),
	}

	stop := func() {
		if err := r.Stop(); err != nil {
			log.Printf("warning: failed to stop recorder: %s", err)
		}
	}

	return factories, stop
}

// testAccPreCheck validates required environment variables are set.
// During replay, SANITY_TOKEN can be a dummy value.
func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("SANITY_TOKEN"); v == "" {
		t.Fatal("SANITY_TOKEN must be set for acceptance tests")
	}
}

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

func replaceAll(s, old, new string) string {
	return regexp.MustCompile(regexp.QuoteMeta(old)).ReplaceAllLiteralString(s, new)
}

var variableBlockRe = regexp.MustCompile(`(?ms)^variable\s+"[^"]+"\s*\{[^}]*\}\s*\n?`)

func removeVariableBlocks(s string) string {
	return variableBlockRe.ReplaceAllString(s, "")
}

// prerequisiteProject returns HCL for a test project.
func prerequisiteProject(name string) string {
	return fmt.Sprintf(`
resource "sanity_project" "prereq" {
  name = %q
}
`, name)
}

// prerequisiteProjectAndDataset returns HCL for a test project + dataset.
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
	f, stop := providerFactoriesWithRecorder("TestAccProject_basic")
	defer stop()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: loadExample(t, "sanity_project", map[string]string{
					"project_name": `"tf-vcr-project"`,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_project.main", "id"),
					resource.TestCheckResourceAttr("sanity_project.main", "name", "tf-vcr-project"),
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
	f, stop := providerFactoriesWithRecorder("TestAccDataset_basic")
	defer stop()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProject("tf-vcr-dataset-project") +
					loadExample(t, "sanity_dataset", map[string]string{
						"project_id":   "sanity_project.prereq.id",
						"dataset_name": `"tfvcr"`,
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sanity_dataset.main", "name", "tfvcr"),
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
	f, stop := providerFactoriesWithRecorder("TestAccCORSOrigin_basic")
	defer stop()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProject("tf-vcr-cors-project") +
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
	f, stop := providerFactoriesWithRecorder("TestAccWebhook_basic")
	defer stop()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProjectAndDataset("tf-vcr-webhook-project", "tfvcrwh") +
					loadExample(t, "sanity_webhook", map[string]string{
						"project_id":     "sanity_project.prereq.id",
						"dataset_name":   `"tfvcrwh"`,
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
	f, stop := providerFactoriesWithRecorder("TestAccSchemaType_basic")
	defer stop()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProjectAndDataset("tf-vcr-schema-project", "tfvcrsc") +
					loadExample(t, "sanity_schema_type", map[string]string{
						"project_id":   "sanity_project.prereq.id",
						"dataset_name": `"tfvcrsc"`,
					}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sanity_schema_type.article", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "name", "article"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "type", "document"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "title", "Article"),
					resource.TestCheckResourceAttr("sanity_schema_type.article", "field.#", "6"),
					resource.TestCheckResourceAttrSet("sanity_schema_type.author", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.author", "name", "author"),
					resource.TestCheckResourceAttrSet("sanity_schema_type.category", "id"),
					resource.TestCheckResourceAttr("sanity_schema_type.category", "name", "category"),
				),
			},
		},
	})
}

// --- Project Token ---

func TestAccProjectToken_basic(t *testing.T) {
	f, stop := providerFactoriesWithRecorder("TestAccProjectToken_basic")
	defer stop()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: prerequisiteProject("tf-vcr-token-project") +
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
