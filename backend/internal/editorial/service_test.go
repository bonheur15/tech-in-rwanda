package editorial

import (
	"context"
	"encoding/json"
	"errors"
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

func TestReviewApprovalPublishesSubmittedVersion(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "review.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO identities(id,email,created_at,updated_at)VALUES('author','review@example.com','now','now'),('admin','admin@example.com','now','now'); INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,status,created_at,updated_at)VALUES('author','reviewer','Reviewer','author','review_required','active','now','now'),('admin','admin','Admin','superadmin','direct_publish','active','now','now')")
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{DB: db}
	author := auth.Actor{IdentityID: "author", Kind: "staff", Role: "author", PublishMode: "review_required", Status: "active"}
	admin := auth.Actor{IdentityID: "admin", Kind: "staff", Role: "superadmin", PublishMode: "direct_publish", Status: "active"}
	doc := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Reviewed evidence"}]}]}`)
	post, err := service.Create(ctx, author, "Reviewed title", "Reviewed excerpt", doc)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.Submit(ctx, author, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Decide(ctx, admin, review, true, "verified"); err != nil {
		t.Fatal(err)
	}
	published, err := service.Public(ctx, "reviewed-title")
	if err != nil || published.Title != "Reviewed title" || string(published.Content) != string(doc) {
		t.Fatalf("published=%+v err=%v", published, err)
	}
	var status string
	if err = db.QueryRow("SELECT status FROM review_decisions WHERE id=?", review).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestAutosaveIsLastSaveWins(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "autosave.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO identities(id,email,created_at,updated_at)VALUES('author','save@example.com','now','now'); INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,status,created_at,updated_at)VALUES('author','saver','Saver','author','direct_publish','active','now','now')")
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{DB: db}
	actor := auth.Actor{IdentityID: "author", Kind: "staff", Role: "author", PublishMode: "direct_publish", Status: "active"}
	first := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]}]}`)
	second := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"last"}]}]}`)
	post, err := service.Create(ctx, actor, "Draft", "", first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Save(ctx, actor, post.ID, "Earlier", "", first, post.Revision); err != nil {
		t.Fatal(err)
	}
	latest, err := service.Save(ctx, actor, post.ID, "Latest", "", second, post.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Title != "Latest" || string(latest.Content) != string(second) {
		t.Fatalf("latest=%+v", latest)
	}
}

func TestDocumentValidationRejectsUnsafeRichContent(t *testing.T) {
	unsafeImage := json.RawMessage(`{"type":"doc","content":[{"type":"image","attrs":{"src":"https://tracker.example/pixel.png","alt":"pixel","placement":"center"}}]}`)
	badHeading := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"No H1"}]}]}`)
	unsafeLink := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"click","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)"}}]}]}]}`)
	for _, document := range []json.RawMessage{unsafeImage, badHeading, unsafeLink} {
		if ValidateDocument(document) == nil {
			t.Fatalf("unsafe document accepted: %s", document)
		}
	}
}

func TestDraftAllowsMissingAltButPublicationReportsImageIssue(t *testing.T) {
	document := json.RawMessage(`{"type":"doc","content":[{"type":"image","attrs":{"assetId":"asset-1","src":"/media/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/large.jpg","alt":"","placement":"small","width":55,"cropAspect":"16:9","focalX":0.25,"focalY":0.75}}]}`)
	if err := ValidateDocumentMode(document, DraftValidation); err != nil {
		t.Fatalf("draft rejected: %v", err)
	}
	var issue *DocumentNotPublishableError
	if err := ValidateDocumentMode(document, PublishValidation); !errors.As(err, &issue) || len(issue.Issues) != 1 || issue.Issues[0].AssetID != "asset-1" {
		t.Fatalf("expected structured missing-alt issue, got %#v", err)
	}
}

func TestImageWidthCropAndFocalBounds(t *testing.T) {
	base := `/media/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/large.jpg`
	for _, attrs := range []string{
		`"placement":"left","width":65,"cropAspect":"original"`,
		`"placement":"center","width":33,"cropAspect":"original"`,
		`"placement":"center","width":50,"cropAspect":"2:1"`,
		`"placement":"center","width":50,"cropAspect":"original","focalX":1.1`,
	} {
		document := json.RawMessage(`{"type":"doc","content":[{"type":"image","attrs":{"src":"` + base + `","alt":"described",` + attrs + `}}]}`)
		if ValidateDocumentMode(document, DraftValidation) == nil {
			t.Fatalf("invalid image accepted: %s", attrs)
		}
	}
}
