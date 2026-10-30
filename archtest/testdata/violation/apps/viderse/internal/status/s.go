// Package status breaks the business rule twice: it imports web and persistence.
package status

import (
	_ "example.com/violation/apps/viderse/internal/domain"
	_ "example.com/violation/apps/viderse/internal/persistence/store"
	_ "example.com/violation/apps/viderse/internal/web"
)
