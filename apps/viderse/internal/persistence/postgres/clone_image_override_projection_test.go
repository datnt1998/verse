package postgres

import (
	"testing"

	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

func TestImageOverrideDisablesAndRestoresScenes(t *testing.T) {
	m := images.Manifest{Version:1, Revision:3, Items:[]images.Item{{ID:"scene", Kind:"scene", Status:"review"}, {ID:"cover", Kind:"thumbnail", Status:"review"}}}
	desired := images.Manifest{ImageMode:"none", Items:[]images.Item{{ID:"cover", Kind:"thumbnail"}}}
	applyImageOverride(&m, desired)
	if len(m.Items)!=1 || m.Items[0].ID!="cover" || m.ImageMode!="none" { t.Fatalf("no-images projection: %+v",m) }
	desired.ImageMode = "auto"
	desired.Items = []images.Item{{ID:"scene",Kind:"scene",Status:"pending"},{ID:"cover",Kind:"thumbnail"}}
	applyImageOverride(&m, desired)
	if len(m.Items)!=2 || m.Items[0].ID!="scene" || m.Items[0].Status!="pending" || m.Items[1].ID!="cover" { t.Fatalf("restore projection: %+v",m) }
}
