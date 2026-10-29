// Package x breaks the core rule: core imports an app.
package x

import (
	_ "example.com/violation/apps/a/y"
	_ "example.com/violation/core/c"
)
