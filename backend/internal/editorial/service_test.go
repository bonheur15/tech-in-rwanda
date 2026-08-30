package editorial

import (
	"context"
	"encoding/json"
	"path/filepath"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/platform/database"
	"testing"
)

func TestPublishedSnapshotSurvivesDraftEdits(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO identities(id,email,created_at,updated_at)VALUES('author','a@example.com','now','now'); INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,status,created_at,updated_at)VALUES('author','author','Author','author','direct_publish','active','now','now')")
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{DB: db}
	actor := auth.Actor{IdentityID: "author", Kind: "staff", Role: "author", PublishMode: "direct_publish", Status: "active"}
	first := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First"}]}]}`)
	post, err := service.Create(ctx, actor, "First title", "first", first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Publish(ctx, actor, post.ID); err != nil {
		t.Fatal(err)
	}
	second := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Second"}]}]}`)
	if _, err = service.Save(ctx, actor, post.ID, "Changed title", "changed", second, post.Revision); err != nil {
		t.Fatal(err)
	}
	published, err := service.Public(ctx, Slug("First title"))
	if err != nil {
		t.Fatal(err)
	}
	if published.Title != "First title" || string(published.Content) != string(first) {
		t.Fatalf("published snapshot changed: %#v", published)
	}
}
