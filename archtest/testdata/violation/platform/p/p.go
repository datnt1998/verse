// Package p breaks the platform rule: platform imports an app.
package p

import (
	_ "example.com/violation/apps/a/y"
	_ "example.com/violation/core/c"
	_ "example.com/violation/platform/q"
)
