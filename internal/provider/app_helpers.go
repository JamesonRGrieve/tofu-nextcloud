// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"strings"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
)

// appID is the resource id: "<path>/<app_id>".
func appID(p, id string) string {
	return strings.TrimRight(p, "/") + "/" + id
}

// parseAppImportID splits a "<path>/<app_id>" import id. A bare app id (no slash)
// leaves path empty so the provider docroot is used.
func parseAppImportID(id string) (pathPart, appIDPart string) {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
}

// appEnableAction maps a desired enabled state to the occ sub-action that
// realizes it. Pure — unit-tested.
func appEnableAction(enabled bool) string {
	if enabled {
		return "enable"
	}
	return "disable"
}

// reconcileAppEnabled drives the app to the desired enabled state (idempotent —
// enabling an already-enabled app is a no-op on the device).
func reconcileAppEnabled(occ *nextcloud.OCC, id string, enabled bool) error {
	switch appEnableAction(enabled) {
	case "enable":
		return occ.AppEnable(id)
	default:
		return occ.AppDisable(id)
	}
}
