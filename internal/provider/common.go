// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// planGetter, attrGetter, and stateSetter abstract the request/response plumbing
// so a resource's Create and Update can share one apply routine. tfsdk.Plan,
// tfsdk.Config, and *tfsdk.State satisfy them respectively.
type planGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

type attrGetter interface {
	GetAttribute(ctx context.Context, p path.Path, target interface{}) diag.Diagnostics
}

type stateSetter interface {
	Set(ctx context.Context, val interface{}) diag.Diagnostics
}

// configureClient extracts the shared *providerClient from a resource Configure
// request, adding a diagnostic on a type mismatch. Returns nil before the
// provider is configured (ProviderData nil), which callers treat as "skip".
func configureClient(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *providerClient {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*providerClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("expected *providerClient, got %T", req.ProviderData))
		return nil
	}
	return client
}

// occ binds an occ wrapper to a docroot over the provider's SSH transport, run
// as the provider's configured web user.
func (c *providerClient) occ(path string) *nextcloud.OCC {
	return nextcloud.NewOCC(c.SSH, path, c.WebUser)
}

// resolvePath returns the resource's declared path, or the provider default
// docroot when the attribute is null/empty.
func resolvePath(path types.String, docroot string) string {
	if !path.IsNull() && path.ValueString() != "" {
		return path.ValueString()
	}
	return docroot
}

// mapValues extracts a Go map from a types.Map (nil-safe: null/unknown → empty).
func mapValues(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	out := map[string]string{}
	if m.IsNull() || m.IsUnknown() {
		return out
	}
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

// sortedKeys returns the map keys in deterministic order (stable applies/tests).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
