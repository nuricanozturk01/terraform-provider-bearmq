package provider

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// argsToMap converts a Terraform string map into the map[string]any BearMQ
// expects for queue/exchange/binding "arguments". Values that parse cleanly as a
// bool or number are sent as that JSON type (so e.g. "x-message-ttl" = "60000"
// reaches the broker as the number 60000); everything else stays a string.
func argsToMap(ctx context.Context, m types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}

	raw := make(map[string]string, len(m.Elements()))
	diags.Append(m.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make(map[string]any, len(raw))
	for k, v := range raw {
		out[k] = coerceScalar(v)
	}
	return out, diags
}

func coerceScalar(v string) any {
	switch strings.ToLower(v) {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

// splitImportID parses a "<parent>/<child>" import identifier.
func splitImportID(id string) (parent, child string, ok bool) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
