// SPDX-License-Identifier: AGPL-3.0-or-later
//
// occ wrapper. All Nextcloud state is read and written by running `occ` on the
// host. Because Nextcloud's data and config must be owned by the web-server user,
// occ is run as that user (`sudo -u www-data php <docroot>/occ …`). The
// command-building helpers are pure (unit-tested); the run methods go through an
// injected Executor so the apply logic is testable without a device and NEVER
// touches a real host in a unit test.
package nextcloud

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Executor runs a remote shell command, optionally piping stdin, and returns
// stdout. *SSHClient satisfies it in production; a fake satisfies it in tests.
type Executor interface {
	Run(remote string, stdin []byte) ([]byte, error)
}

// DefaultWebUser is the web-server user occ runs as when none is configured.
const DefaultWebUser = "www-data"

// OCC drives occ for a single Nextcloud install rooted at Path, run as WebUser.
type OCC struct {
	Exec    Executor
	Path    string // Nextcloud document root (holds occ)
	WebUser string // web-server user occ runs as (e.g. www-data)
}

// NewOCC binds an occ wrapper to an executor, docroot, and web user. An empty
// webUser defaults to DefaultWebUser.
func NewOCC(exec Executor, path, webUser string) *OCC {
	if webUser == "" {
		webUser = DefaultWebUser
	}
	return &OCC{Exec: exec, Path: path, WebUser: webUser}
}

// shQuote single-quotes a value for safe use in a remote POSIX shell command.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// occPath returns the path to the occ script for a docroot (`<docroot>/occ`).
func occPath(docroot string) string {
	return strings.TrimRight(docroot, "/") + "/occ"
}

// occCommand renders an occ invocation string: run as the web user via sudo,
// invoke php against the install's occ script, and pass the quoted sub-arguments.
// Pure — unit-tested.
func occCommand(webUser, path string, args ...string) string {
	parts := []string{"sudo", "-u", shQuote(webUser), "php", shQuote(occPath(path))}
	for _, a := range args {
		parts = append(parts, shQuote(a))
	}
	return strings.Join(parts, " ")
}

func (o *OCC) run(args ...string) ([]byte, error) {
	out, err := o.Exec.Run(occCommand(o.WebUser, o.Path, args...), nil)
	if err != nil {
		return nil, fmt.Errorf("occ %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Command runs an arbitrary occ invocation and returns trimmed stdout. It is the
// escape hatch resources use for commands without a dedicated method
// (`maintenance:install`, `upgrade`, …). Every argument is shell-quoted.
func (o *OCC) Command(args ...string) (string, error) {
	out, err := o.run(args...)
	if err != nil {
		return "", err
	}
	return trimOut(out), nil
}

// trimOut returns stdout trimmed of surrounding whitespace/newlines.
func trimOut(b []byte) string { return strings.TrimSpace(string(b)) }

// StatusInfo is the parsed subset of `occ status --output=json`.
type StatusInfo struct {
	Installed bool
	Version   string // dotted version string (e.g. "28.0.1")
}

// Status returns the parsed `occ status --output=json`. A non-installed instance
// still returns a StatusInfo with Installed=false (occ status exits 0 either way).
func (o *OCC) Status() (StatusInfo, error) {
	out, err := o.run(statusArgs()...)
	if err != nil {
		return StatusInfo{}, err
	}
	return parseStatus(out)
}

// IsInstalled reports whether the instance is installed (`occ status`).
func (o *OCC) IsInstalled() bool {
	s, err := o.Status()
	return err == nil && s.Installed
}

// CoreVersion returns the installed Nextcloud version (from `occ status`).
func (o *OCC) CoreVersion() (string, error) {
	s, err := o.Status()
	if err != nil {
		return "", err
	}
	return s.Version, nil
}

// ConfigSystemGet returns a single system config value (`occ config:system:get`).
// A missing key makes occ exit non-zero → returned as ("", err).
func (o *OCC) ConfigSystemGet(key string) (string, error) {
	out, err := o.run(configSystemGetArgs(key)...)
	if err != nil {
		return "", err
	}
	return trimOut(out), nil
}

// ConfigSystemSet writes a system config value (`occ config:system:set`). The
// valueType is occ's `--type` (string/integer/boolean/json/null); an empty or
// "string" type omits the flag.
func (o *OCC) ConfigSystemSet(key, value, valueType string) error {
	_, err := o.run(configSystemSetArgs(key, value, valueType)...)
	return err
}

// ConfigSystemDelete removes a system config key (`occ config:system:delete`).
func (o *OCC) ConfigSystemDelete(key string) error {
	_, err := o.run(configSystemDeleteArgs(key)...)
	return err
}

// ConfigAppGet returns a single app config value (`occ config:app:get`).
func (o *OCC) ConfigAppGet(app, key string) (string, error) {
	out, err := o.run(configAppGetArgs(app, key)...)
	if err != nil {
		return "", err
	}
	return trimOut(out), nil
}

// ConfigAppSet writes an app config value (`occ config:app:set`).
func (o *OCC) ConfigAppSet(app, key, value string) error {
	_, err := o.run(configAppSetArgs(app, key, value)...)
	return err
}

// ConfigAppDelete removes an app config key (`occ config:app:delete`).
func (o *OCC) ConfigAppDelete(app, key string) error {
	_, err := o.run(configAppDeleteArgs(app, key)...)
	return err
}

// AppList returns the parsed `occ app:list --output=json` — every app keyed by
// id with its enabled flag and version.
func (o *OCC) AppList() (map[string]AppState, error) {
	out, err := o.run(appListArgs()...)
	if err != nil {
		return nil, err
	}
	return parseAppList(out)
}

// statusArgs lists the instance status as JSON (`occ status --output=json`).
func statusArgs() []string {
	return []string{"status", "--output=json"}
}

// parseStatus extracts installed/version from `occ status --output=json`.
func parseStatus(b []byte) (StatusInfo, error) {
	var raw struct {
		Installed     bool   `json:"installed"`
		Version       string `json:"version"`
		VersionString string `json:"versionstring"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return StatusInfo{}, fmt.Errorf("parse occ status json: %w", err)
	}
	version := raw.VersionString
	if version == "" {
		version = raw.Version
	}
	return StatusInfo{Installed: raw.Installed, Version: version}, nil
}
