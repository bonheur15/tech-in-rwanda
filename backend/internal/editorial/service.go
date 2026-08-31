package editorial

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"regexp"
	"rwandafreespace.com/blog/backend/internal/auth"
	"strings"
	"time"
)

type Service struct {
	DB  *sql.DB
	Now func() time.Time
}
type Post struct {
	ID                 string          `json:"id"`
	OwnerID            string          `json:"ownerId"`
	Title              string          `json:"title"`
	Slug               string          `json:"slug"`
	Excerpt            string          `json:"excerpt"`
	State              string          `json:"state"`
	Content            json.RawMessage `json:"content"`
	Revision           int             `json:"revision"`
	PublishedVersionID *string         `json:"publishedVersionId"`
	SourcePostID       *string         `json:"sourcePostId"`
	UpdatedAt          string          `json:"updatedAt"`
	PublishedAt        *string         `json:"publishedAt"`
}

var slugClean = regexp.MustCompile(`[^a-z0-9]+`)

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func Slug(v string) string {
	v = strings.Trim(slugClean.ReplaceAllString(strings.ToLower(v), "-"), "-")
	if v == "" {
		v = "untitled"
	}
	return v
}
func ValidateDocument(raw json.RawMessage) error {
	if len(raw) > 2<<20 {
		return errors.New("document is too large")
	}
	type mark struct {
		Type  string         `json:"type"`
		Attrs map[string]any `json:"attrs,omitempty"`
	}
	type node struct {
		Type    string            `json:"type"`
		Text    string            `json:"text,omitempty"`
		Attrs   map[string]any    `json:"attrs,omitempty"`
		Marks   []mark            `json:"marks,omitempty"`
		Content []json.RawMessage `json:"content,omitempty"`
	}
	var root node
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&root) != nil || root.Type != "doc" {
		return errors.New("content must be a TipTap document")
	}
	allowed := map[string]bool{"paragraph": true, "heading": true, "bulletList": true, "orderedList": true, "listItem": true, "blockquote": true, "codeBlock": true, "horizontalRule": true, "image": true, "text": true}
	count := 0
	var validate func(json.RawMessage, int) error
	validate = func(body json.RawMessage, depth int) error {
		count++
		if count > 20_000 || depth > 20 {
			return errors.New("document structure is too complex")
		}
		var current node
		d := json.NewDecoder(bytes.NewReader(body))
		d.DisallowUnknownFields()
		if d.Decode(&current) != nil || !allowed[current.Type] {
			return fmt.Errorf("unsupported content node")
		}
		if current.Type == "heading" {
			level, ok := current.Attrs["level"].(float64)
			if !ok || (level != 2 && level != 3) {
				return errors.New("only H2 and H3 headings are supported")
			}
		}
		if current.Type == "image" {
			src, _ := current.Attrs["src"].(string)
			alt, _ := current.Attrs["alt"].(string)
			placement, _ := current.Attrs["placement"].(string)
			if !regexp.MustCompile(`^/media/[a-f0-9]{64}/(original|large|medium|small)\.jpg$`).MatchString(src) || strings.TrimSpace(alt) == "" {
				return errors.New("images require a managed media URL and alternative text")
			}
			if placement != "" && !map[string]bool{"center": true, "wide": true, "full": true, "left": true, "right": true}[placement] {
				return errors.New("unsupported image placement")
			}
		}
		for _, m := range current.Marks {
			if !map[string]bool{"bold": true, "italic": true, "code": true, "link": true}[m.Type] {
				return errors.New("unsupported text formatting")
			}
			if m.Type == "link" {
				href, _ := m.Attrs["href"].(string)
				if !(strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "/") || strings.HasPrefix(href, "#")) {
					return errors.New("unsupported link URL")
				}
			}
		}
		for _, child := range current.Content {
			if err := validate(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range root.Content {
		if err := validate(child, 1); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Create(ctx context.Context, a auth.Actor, title, excerpt string, content json.RawMessage) (Post, error) {
	if a.Kind != "staff" || a.Status != "active" {
		return Post{}, auth.ErrUnauthorized
	}
	if err := ValidateDocument(content); err != nil {
		return Post{}, err
	}
	id := uuid.NewString()
	slug := Slug(title) + "-" + id[:8]
	now := s.now().Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO posts(id,owner_id,title,slug,excerpt,state,created_at,updated_at)VALUES(?,?,?,?,?,'draft',?,?)", id, a.IdentityID, title, slug, excerpt, now, now); err != nil {
		return Post{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO post_drafts(post_id,content_json,updated_at)VALUES(?,?,?)", id, string(content), now); err != nil {
		return Post{}, err
	}
	if err = tx.Commit(); err != nil {
		return Post{}, err
	}
	return s.GetDraft(ctx, a, id)
}
func (s *Service) GetDraft(ctx context.Context, a auth.Actor, id string) (Post, error) {
	var p Post
	var raw string
	err := s.DB.QueryRowContext(ctx, "SELECT p.id,p.owner_id,p.title,p.slug,p.excerpt,p.state,d.content_json,d.revision,p.published_version_id,p.source_post_id,p.updated_at,p.published_at FROM posts p JOIN post_drafts d ON d.post_id=p.id WHERE p.id=?", id).Scan(&p.ID, &p.OwnerID, &p.Title, &p.Slug, &p.Excerpt, &p.State, &raw, &p.Revision, &p.PublishedVersionID, &p.SourcePostID, &p.UpdatedAt, &p.PublishedAt)
	if err != nil {
		return p, err
	}
	if a.Role != "superadmin" && p.OwnerID != a.IdentityID {
		return p, auth.ErrUnauthorized
	}
	p.Content = json.RawMessage(raw)
	return p, nil
}
func (s *Service) Save(ctx context.Context, a auth.Actor, id, title, excerpt string, content json.RawMessage, _ int) (Post, error) {
	if err := ValidateDocument(content); err != nil {
		return Post{}, err
	}
	p, err := s.GetDraft(ctx, a, id)
	if err != nil {
		return Post{}, err
	}
	if p.State == "frozen" {
		return Post{}, errors.New("draft is frozen")
	}
	now := s.now().Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "UPDATE post_drafts SET content_json=?,revision=revision+1,updated_at=? WHERE post_id=?", string(content), now, id)
	if err != nil {
		return Post{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Post{}, sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, "UPDATE posts SET title=?,excerpt=?,updated_at=?,state=CASE WHEN state='in_review' THEN 'draft' ELSE state END WHERE id=?", title, excerpt, now, id); err != nil {
		return Post{}, err
	}
	tx.ExecContext(ctx, "UPDATE review_decisions SET status='cancelled',decided_at=? WHERE post_id=? AND status='pending'", now, id)
	if err = tx.Commit(); err != nil {
		return Post{}, err
	}
	return s.GetDraft(ctx, a, id)
}
func (s *Service) Checkpoint(ctx context.Context, a auth.Actor, id, reason string) (string, error) {
	p, err := s.GetDraft(ctx, a, id)
	if err != nil {
		return "", err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	return checkpoint(ctx, tx, a, p, reason, s.now())
}
func checkpoint(ctx context.Context, tx *sql.Tx, a auth.Actor, p Post, reason string, now time.Time) (string, error) {
	var number int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(number),0)+1 FROM post_versions WHERE post_id=?", p.ID).Scan(&number); err != nil {
		return "", err
	}
	id := uuid.NewString()
	_, err := tx.ExecContext(ctx, "INSERT INTO post_versions(id,post_id,number,content_json,title,excerpt,reason,created_by,created_at)VALUES(?,?,?,?,?,?,?,?,?)", id, p.ID, number, string(p.Content), p.Title, p.Excerpt, reason, a.IdentityID, now.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}
func (s *Service) Submit(ctx context.Context, a auth.Actor, id string) (string, error) {
	if a.PublishMode != "review_required" {
		return "", errors.New("account does not require review")
	}
	version, err := s.Checkpoint(ctx, a, id, "review submission")
	if err != nil {
		return "", err
	}
	now := s.now().Format(time.RFC3339Nano)
	decision := uuid.NewString()
	_, err = s.DB.ExecContext(ctx, "INSERT INTO review_decisions(id,post_id,version_id,submitted_by,status,created_at)VALUES(?,?,?,?,'pending',?); UPDATE posts SET state='in_review',updated_at=? WHERE id=?", decision, id, version, a.IdentityID, now, now, id)
	return decision, err
}
func (s *Service) Publish(ctx context.Context, a auth.Actor, id string) (string, error) {
	if a.Role != "superadmin" && a.PublishMode != "direct_publish" {
		return "", auth.ErrUnauthorized
	}
	version, err := s.Checkpoint(ctx, a, id, "publication")
	if err != nil {
		return "", err
	}
	return version, s.publishVersion(ctx, a, id, version)
}
func (s *Service) Decide(ctx context.Context, a auth.Actor, id string, approve bool, reason string) error {
	if a.Role != "superadmin" {
		return auth.ErrUnauthorized
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var postID, version string
	if err := tx.QueryRowContext(ctx, "SELECT post_id,version_id FROM review_decisions WHERE id=? AND status='pending'", id).Scan(&postID, &version); err != nil {
		return err
	}
	if approve {
		if err := s.publishVersionTx(ctx, tx, postID, version); err != nil {
			return err
		}
	}
	status := "rejected"
	if approve {
		status = "approved"
	}
	now := s.now().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, "UPDATE review_decisions SET status=?,reason=?,decided_by=?,decided_at=? WHERE id=? AND status='pending'; UPDATE posts SET state=CASE WHEN ?='approved' THEN 'published' ELSE 'draft' END WHERE id=?", status, reason, a.IdentityID, now, id, status, postID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) publishVersion(ctx context.Context, a auth.Actor, postID, version string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.publishVersionTx(ctx, tx, postID, version); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) publishVersionTx(ctx context.Context, tx *sql.Tx, postID, version string) error {
	var title, excerpt, body, oldSlug string
	if err := tx.QueryRowContext(ctx, "SELECT v.title,v.excerpt,v.content_json,p.slug FROM post_versions v JOIN posts p ON p.id=v.post_id WHERE v.id=? AND v.post_id=?", version, postID).Scan(&title, &excerpt, &body, &oldSlug); err != nil {
		return err
	}
	now := s.now().Format(time.RFC3339Nano)
	slug := Slug(title)
	var n int
	_ = tx.QueryRowContext(ctx, "SELECT count(*) FROM posts WHERE slug=? AND id<>?", slug, postID).Scan(&n)
	if n > 0 {
		slug = slug + "-" + postID[:8]
	}
	if oldSlug != slug {
		tx.ExecContext(ctx, "INSERT OR IGNORE INTO slug_aliases(slug,post_id,created_at)VALUES(?,?,?)", oldSlug, postID, now)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE posts SET title=?,excerpt=?,slug=?,published_version_id=?,state='published',ever_published=1,published_at=COALESCE(published_at,?),updated_at=? WHERE id=?", title, excerpt, slug, version, now, now, postID); err != nil {
		return err
	}
	tx.ExecContext(ctx, "DELETE FROM post_search WHERE post_id=?", postID)
	tx.ExecContext(ctx, "INSERT INTO post_search(post_id,title,excerpt,body)VALUES(?,?,?,?)", postID, title, excerpt, body)
	tx.ExecContext(ctx, "UPDATE media_assets SET status='public' WHERE id IN(SELECT asset_id FROM article_media WHERE post_id=?)", postID)
	return nil
}
func (s *Service) Fork(ctx context.Context, a auth.Actor, postID string) (Post, error) {
	var title, excerpt, content, version string
	if err := s.DB.QueryRowContext(ctx, "SELECT v.title,v.excerpt,v.content_json,v.id FROM posts p JOIN post_versions v ON v.id=p.published_version_id WHERE p.id=?", postID).Scan(&title, &excerpt, &content, &version); err != nil {
		return Post{}, err
	}
	p, err := s.Create(ctx, a, "Fork of "+title, excerpt, json.RawMessage(content))
	if err != nil {
		return p, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "UPDATE posts SET source_post_id=?,source_version_id=?,category_id=(SELECT category_id FROM posts WHERE id=?),thumbnail_asset_id=(SELECT thumbnail_asset_id FROM posts WHERE id=?) WHERE id=?", postID, version, postID, postID, p.ID)
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO post_tags(post_id,tag_id) SELECT ?,tag_id FROM post_tags WHERE post_id=?", p.ID, postID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO article_media(post_id,asset_id,placement) SELECT ?,asset_id,placement FROM article_media WHERE post_id=?", p.ID, postID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err == nil {
		_, err = s.Checkpoint(ctx, a, p.ID, "fork")
	}
	return p, err
}
func (s *Service) Public(ctx context.Context, slug string) (Post, error) {
	var p Post
	var content string
	err := s.DB.QueryRowContext(ctx, "SELECT p.id,p.owner_id,v.title,p.slug,v.excerpt,p.state,v.content_json,0,p.published_version_id,p.source_post_id,p.updated_at,p.published_at FROM posts p JOIN post_versions v ON v.id=p.published_version_id WHERE (p.slug=? OR p.id=(SELECT post_id FROM slug_aliases WHERE slug=?)) AND p.state='published'", slug, slug).Scan(&p.ID, &p.OwnerID, &p.Title, &p.Slug, &p.Excerpt, &p.State, &content, &p.Revision, &p.PublishedVersionID, &p.SourcePostID, &p.UpdatedAt, &p.PublishedAt)
	p.Content = json.RawMessage(content)
	return p, err
}
func (s *Service) Latest(ctx context.Context, limit int) ([]Post, error) {
	if limit < 1 || limit > 50 {
		limit = 12
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT p.id,p.owner_id,v.title,p.slug,v.excerpt,p.state,'{}',0,p.published_version_id,p.source_post_id,p.updated_at,p.published_at FROM posts p JOIN post_versions v ON v.id=p.published_version_id WHERE p.state='published' ORDER BY p.published_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		if err = rows.Scan(&p.ID, &p.OwnerID, &p.Title, &p.Slug, &p.Excerpt, &p.State, &p.Content, &p.Revision, &p.PublishedVersionID, &p.SourcePostID, &p.UpdatedAt, &p.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
