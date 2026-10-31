package web

import (
 "context"
 "testing"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func TestQueuedPromptOverridesPersistPG(t *testing.T) {
 _, pool := planFixture(t)
 ctx := context.Background()
 _, err := pool.Exec(ctx, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,source_snapshot,position) VALUES ('33333333-3333-4333-8333-333333333333','11111111-1111-4111-8111-111111111111','aaaaaaaaaaa','queued-prompt-test','{}',1)`)
 if err != nil { t.Fatal(err) }
 repo := postgres.NewQueueStore(pool)
 item, err := repo.GetQueueItem(ctx, "33333333-3333-4333-8333-333333333333")
 if err != nil || item == nil { t.Fatalf("read queue row: %+v %v", item, err) }
 scene, cover, noImages := "builtin:scene", "builtin:cover-clean", true
 item.ScenePromptID, item.CoverPromptID, item.NoImages, item.NoImagesOverride = &scene, &cover, true, &noImages
 ok, err := repo.UpdateQueueOverrides(ctx, *item, item.OverrideRevision)
 if err != nil || !ok { t.Fatalf("update: %v %v", ok, err) }
 item, err = postgres.NewQueueStore(pool).GetQueueItem(ctx, item.ID)
 if err != nil || item == nil || item.ScenePromptID == nil || *item.ScenePromptID != scene || item.CoverPromptID == nil || *item.CoverPromptID != cover || !item.NoImages || item.NoImagesOverride == nil || !*item.NoImagesOverride || item.OverrideRevision != 2 { t.Fatalf("stored overrides: %+v %v", item, err) }
}
