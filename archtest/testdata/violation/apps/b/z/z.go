// Package z breaks the app rule: one app imports another.
package z

import (
	_ "example.com/violation/apps/a/y"
	_ "example.com/violation/core/c"
	_ "example.com/violation/platform/q"
)
