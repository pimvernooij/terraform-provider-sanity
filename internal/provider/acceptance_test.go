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
func providerFactoriesWithRecorder(t *testing.T, cassetteName string) (map[string]func() (tfprotov6.ProviderServer, error), func()) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set, skipping acceptance test")
	}

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
		t.Fatalf("failed to create VCR recorder for cassette %s: %s", cassetteName, err)
	}

	// When recording, set the oauth2 transport as real transport so requests
	// are authenticated. During replay this is unused.
	if mode == recorder.ModeRecordOnly {
		token := os.Getenv("SANITY_TOKEN")
		if token == "" {
			t.Fatal("SANITY_TOKEN must be set when RECORD=true")
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

// --- Project (standalone) ---

// TestAccProject_basic tests project creation and import in isolation.
func TestAccProject_basic(t *testing.T) {
	f, stop := providerFactoriesWithRecorder(t, "TestAccProject_basic")
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

// --- All resources within a single project ---

// testAccBaseConfig returns HCL for the shared project that all child
// resources depend on. This mirrors real-world usage: one project
// containing datasets, CORS origins, webhooks, tokens, and schemas.
func testAccBaseConfig() string {
	return `
resource "sanity_project" "test" {
  name = "tf-vcr-project"
}
`
}

// TestAccProjectResources tests all resource types within a single project.
// One cassette, one project — matching real-world usage.
func TestAccProjectResources(t *testing.T) {
	f, stop := providerFactoriesWithRecorder(t, "TestAccProjectResources")
	defer stop()

	vars := map[string]string{
		"project_id":   "sanity_project.test.id",
		"dataset_name": "sanity_dataset.main.name",
	}

	webhookVars := map[string]string{
		"project_id":     "sanity_project.test.id",
		"dataset_name":   "sanity_dataset.main.name",
		"webhook_secret": `"test-secret-123"`,
	}

	// The dataset example creates sanity_dataset.main. Webhook, schema_type,
	// and other resources that need a dataset reference it via its name.
	config := testAccBaseConfig() +
		loadExample(t, "sanity_dataset", map[string]string{
			"project_id":   "sanity_project.test.id",
			"dataset_name": `"tfvcr"`,
		}) +
		loadExample(t, "sanity_cors_origin", vars) +
		loadExample(t, "sanity_webhook", webhookVars) +
		loadExample(t, "sanity_project_token", vars) +
		loadExample(t, "sanity_schema_type", vars)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: f,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Project
					resource.TestCheckResourceAttrSet("sanity_project.test", "id"),
					resource.TestCheckResourceAttr("sanity_project.test", "name", "tf-vcr-project"),

					// Dataset (from example)
					resource.TestCheckResourceAttr("sanity_dataset.main", "name", "tfvcr"),
					resource.TestCheckResourceAttr("sanity_dataset.main", "acl_mode", "public"),

					// CORS Origin (from example)
					resource.TestCheckResourceAttrSet("sanity_cors_origin.main", "id"),
					resource.TestCheckResourceAttr("sanity_cors_origin.main", "origin", "https://example.com"),
					resource.TestCheckResourceAttr("sanity_cors_origin.main", "allow_credentials", "true"),

					// Webhook (from example)
					resource.TestCheckResourceAttrSet("sanity_webhook.main", "id"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "name", "Content Updates Webhook"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "url", "https://api.example.com/webhooks/sanity"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "http_method", "POST"),
					resource.TestCheckResourceAttr("sanity_webhook.main", "filter", "_type == 'post'"),

					// Project Token (from example)
					resource.TestCheckResourceAttrSet("sanity_project_token.main", "id"),
					resource.TestCheckResourceAttr("sanity_project_token.main", "label", "Deployer token"),
					resource.TestCheckResourceAttrSet("sanity_project_token.main", "key"),

					// Schema Types (from example — article, author, category)
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
			// Import: dataset
			{
				ResourceName:      "sanity_dataset.main",
				ImportState:       true,
				ImportStateIdFunc: testAccImportID("sanity_project.test", "sanity_dataset.main", "name"),
				ImportStateVerify: true,
			},
			// Import: webhook
			{
				ResourceName:            "sanity_webhook.main",
				ImportState:             true,
				ImportStateIdFunc:       testAccImportID("sanity_project.test", "sanity_webhook.main", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

// testAccImportID builds a "project_id/resource_attr" import identifier.
// attrKey is the attribute to read from the resource state ("id" or "name").
func testAccImportID(projectRes, targetRes, attrKey string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		project, ok := s.RootModule().Resources[projectRes]
		if !ok {
			return "", fmt.Errorf("resource %s not found", projectRes)
		}
		target, ok := s.RootModule().Resources[targetRes]
		if !ok {
			return "", fmt.Errorf("resource %s not found", targetRes)
		}
		val := target.Primary.Attributes[attrKey]
		if attrKey == "id" {
			val = target.Primary.ID
		}
		return fmt.Sprintf("%s/%s", project.Primary.ID, val), nil
	}
}
