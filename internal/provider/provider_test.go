package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProvider_Schema(t *testing.T) {
	var resp provider.SchemaResponse
	New("test")().Schema(context.Background(), provider.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"endpoint", "api_key", "token", "insecure"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Fatalf("provider schema missing %q", attr)
		}
	}
}

func TestProvider_ResourceSchemasHaveNoDiagnostics(t *testing.T) {
	for _, factory := range New("test")().Resources(context.Background()) {
		res := factory()
		var resp resource.SchemaResponse
		res.Schema(context.Background(), resource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%T schema diagnostics: %v", res, resp.Diagnostics)
		}
		if len(resp.Schema.Attributes) == 0 {
			t.Fatalf("%T produced an empty schema", res)
		}
	}
}

func TestProvider_DataSourceSchemasHaveNoDiagnostics(t *testing.T) {
	for _, factory := range New("test")().DataSources(context.Background()) {
		ds := factory()
		var resp datasource.SchemaResponse
		ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%T schema diagnostics: %v", ds, resp.Diagnostics)
		}
	}
}

func TestProvider_ResourceTypeNames(t *testing.T) {
	want := map[string]bool{
		"bearmq_vhost": false, "bearmq_queue": false,
		"bearmq_exchange": false, "bearmq_binding": false,
	}
	for _, factory := range New("test")().Resources(context.Background()) {
		var resp resource.MetadataResponse
		factory().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "bearmq"}, &resp)
		seen, ok := want[resp.TypeName]
		if !ok {
			t.Fatalf("unexpected resource type name %q", resp.TypeName)
		}
		if seen {
			t.Fatalf("duplicate resource type name %q", resp.TypeName)
		}
		want[resp.TypeName] = true
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("resource %q not registered", name)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Fatalf("got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSplitImportID(t *testing.T) {
	if p, c, ok := splitImportID("vh1/q2"); !ok || p != "vh1" || c != "q2" {
		t.Fatalf("got %q %q %v", p, c, ok)
	}
	if _, _, ok := splitImportID("noseparator"); ok {
		t.Fatal("expected failure on missing separator")
	}
	if _, _, ok := splitImportID("/q2"); ok {
		t.Fatal("expected failure on empty parent")
	}
}

func TestCoerceScalar(t *testing.T) {
	if v := coerceScalar("60000"); v != int64(60000) {
		t.Fatalf("int coercion: %#v", v)
	}
	if v := coerceScalar("true"); v != true {
		t.Fatalf("bool coercion: %#v", v)
	}
	if v := coerceScalar("all"); v != "all" {
		t.Fatalf("string passthrough: %#v", v)
	}
}
