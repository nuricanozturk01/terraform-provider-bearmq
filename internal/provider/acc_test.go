package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories serves the provider in-process for acceptance
// tests. Set once; each test wires its own fake-broker endpoint via the
// per-step provider config block (see providerBlock).
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"bearmq": providerserver.NewProtocol6WithError(New("test")()),
}

// acceptancePreCheck skips acceptance tests unless TF_ACC is set, matching the
// upstream convention. They still need no external services — the fake broker in
// fakebroker_test.go is the backend — but a real `terraform` binary must be on
// PATH, which TF_ACC signals the caller has arranged.
func acceptancePreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests (needs a terraform binary on PATH)")
	}
}

// providerBlock renders a provider config pinned at the fake broker's URL.
func providerBlock(endpoint string) string {
	return fmt.Sprintf(`
provider "bearmq" {
  endpoint = %q
  api_key  = "acctest-key"
}
`, endpoint)
}

// isolateEnv ensures ambient BEARMQ_* env vars don't leak into a test run.
func isolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"BEARMQ_ENDPOINT", "BEARMQ_API_KEY", "BEARMQ_TOKEN", "BEARMQ_INSECURE"} {
		t.Setenv(k, "")
	}
}
