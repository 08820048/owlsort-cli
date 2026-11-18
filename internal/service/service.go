// Package service installs `owlsort watch` as a user-level background service
// (milestone M3): a launchd agent on macOS, a systemd user unit on Linux, a
// scheduled task at logon on Windows. Never needs administrator rights.
package service

import "errors"

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// Install, Uninstall and Status. Milestone M3.
func Install() error          { return ErrNotImplemented }
func Uninstall() error        { return ErrNotImplemented }
func Status() (string, error) { return "", ErrNotImplemented }
