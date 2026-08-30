package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/editorial"
	"rwandafreespace.com/blog/backend/internal/platform/httpx"
	"strconv"
	"strings"
	"time"
)

type API struct {
	DB        *sql.DB
	Auth      *auth.Service
	Editorial *editorial.Service
	Logger    *slog.Logger
	Origin    string
	MediaDir  string
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", a.health)
	mux.HandleFunc("POST /api/v1/auth/{kind}/request-otp", a.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/{kind}/verify-otp", a.verifyOTP)
	mux.HandleFunc("GET /api/v1/sessions", a.withActor(a.sessions))
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", a.withMutation(a.revokeSession))
	mux.HandleFunc("POST /api/v1/reader/onboarding", a.withMutation(a.onboard))
	mux.HandleFunc("GET /api/v1/public/posts", a.publicPosts)
	mux.HandleFunc("GET /api/v1/public/posts/{slug}", a.publicPost)
	mux.HandleFunc("GET /api/v1/search", a.search)
	mux.HandleFunc("POST /api/v1/posts", a.withMutation(a.createPost))
	mux.HandleFunc("GET /api/v1/posts/{id}/draft", a.withActor(a.getDraft))
	mux.HandleFunc("PUT /api/v1/posts/{id}/draft", a.withMutation(a.saveDraft))
	mux.HandleFunc("POST /api/v1/posts/{id}/checkpoint", a.withMutation(a.checkpoint))
	mux.HandleFunc("POST /api/v1/posts/{id}/submit", a.withMutation(a.submit))
	mux.HandleFunc("POST /api/v1/posts/{id}/publish", a.withMutation(a.publish))
	mux.HandleFunc("POST /api/v1/posts/{id}/fork", a.withMutation(a.fork))
	mux.HandleFunc("GET /api/v1/posts/{id}/versions", a.withActor(a.versions))
	mux.HandleFunc("POST /api/v1/posts/{id}/restore/{version}", a.withMutation(a.restoreVersion))
	mux.HandleFunc("DELETE /api/v1/posts/{id}", a.withMutation(a.deletePost))
	mux.HandleFunc("POST /api/v1/reviews/{id}/approve", a.withMutation(func(w http.ResponseWriter, r *http.Request, actor auth.Actor) { a.review(w, r, actor, true) }))
	mux.HandleFunc("POST /api/v1/reviews/{id}/reject", a.withMutation(func(w http.ResponseWriter, r *http.Request, actor auth.Actor) { a.review(w, r, actor, false) }))
	mux.HandleFunc("POST /api/v1/articles/{id}/bookmarks", a.withMutation(a.bookmark))
	mux.HandleFunc("DELETE /api/v1/articles/{id}/bookmarks", a.withMutation(a.unbookmark))
	mux.HandleFunc("GET /api/v1/bookmarks", a.withActor(a.bookmarks))
	mux.HandleFunc("GET /api/v1/articles/{id}/comments", a.comments)
	mux.HandleFunc("POST /api/v1/articles/{id}/comments", a.withMutation(a.addComment))
	mux.HandleFunc("POST /api/v1/admin/comments/{id}/{action}", a.withMutation(a.moderate))
	mux.HandleFunc("GET /api/v1/admin/staff", a.withActor(a.staff))
	mux.HandleFunc("POST /api/v1/admin/staff", a.withMutation(a.addStaff))
	mux.HandleFunc("PATCH /api/v1/admin/staff/{id}", a.withMutation(a.updateStaff))
	mux.HandleFunc("POST /api/v1/media", a.withMutation(a.uploadMedia))
	mux.HandleFunc("GET /media/{hash}/{file}", a.serveMedia)
	return mux
}

type handler func(http.ResponseWriter, *http.Request, auth.Actor)
type actorKey struct{}

func (a *API) withActor(next handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		for _, name := range []string{auth.StaffCookie, auth.ReaderCookie, "rfs_staff_session", "rfs_reader_session"} {
			if c, e := r.Cookie(name); e == nil {
				token = c.Value
				break
			}
		}
		actor, err := a.Auth.Authenticate(r.Context(), token)
		if err != nil {
			httpx.Failure(w, r, 401, "unauthorized", "Sign in is required")
			return
		}
		next(w, r, actor)
	}
}
func (a *API) withMutation(next handler) http.HandlerFunc {
	return a.withActor(func(w http.ResponseWriter, r *http.Request, actor auth.Actor) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if origin != "" && origin != a.Origin {
			httpx.Failure(w, r, 403, "invalid_origin", "The request origin is not allowed")
			return
		}
		if !a.Auth.ValidateCSRF(actor, r.Header.Get("X-CSRF-Token")) {
			httpx.Failure(w, r, 403, "invalid_csrf", "The CSRF token is invalid")
			return
		}
		next(w, r, actor)
	})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 3<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := a.DB.PingContext(ctx); err != nil {
		httpx.Failure(w, r, 503, "database_unavailable", "Database health check failed")
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "ok"})
}
func (a *API) requestOTP(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "staff" && kind != "readers" {
		http.NotFound(w, r)
		return
	}
	if kind == "readers" {
		kind = "reader"
	}
	var in struct{ Email, TurnstileToken string }
	if decode(r, &in) != nil {
		httpx.Failure(w, r, 400, "invalid_request", "Invalid request body")
		return
	}
	err := a.Auth.RequestOTP(r.Context(), kind, in.Email, auth.ClientIP(r), in.TurnstileToken)
	if err != nil && kind == "reader" {
		httpx.Failure(w, r, 429, "request_rejected", err.Error())
		return
	}
	httpx.JSON(w, 202, map[string]bool{"accepted": true})
}
func (a *API) verifyOTP(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind == "readers" {
		kind = "reader"
	}
	var in struct{ Email, Code string }
	if decode(r, &in) != nil {
		httpx.Failure(w, r, 400, "invalid_request", "Invalid request body")
		return
	}
	token, csrf, actor, err := a.Auth.VerifyOTP(r.Context(), kind, in.Email, in.Code, r.UserAgent(), auth.ClientIP(r))
	if err != nil {
		httpx.Failure(w, r, 401, "invalid_code", "The code is invalid or expired")
		return
	}
	auth.SetSessionCookie(w, kind, token, csrf, a.Auth.SecureCookies)
	httpx.JSON(w, 200, map[string]any{"kind": kind, "identityId": actor.IdentityID, "csrfToken": csrf})
}
func (a *API) sessions(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id,user_agent,ip_address,created_at,last_activity_at,expires_at FROM sessions WHERE identity_id=? AND kind=? AND revoked_at IS NULL ORDER BY created_at DESC", x.IdentityID, x.Kind)
	if err != nil {
		httpx.Failure(w, r, 500, "database_error", "Could not load sessions")
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, ua, ip, created, last, expires string
		rows.Scan(&id, &ua, &ip, &created, &last, &expires)
		out = append(out, map[string]string{"id": id, "device": ua, "ipAddress": ip, "createdAt": created, "lastActivityAt": last, "expiresAt": expires})
	}
	httpx.JSON(w, 200, out)
}
func (a *API) revokeSession(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	a.DB.ExecContext(r.Context(), "UPDATE sessions SET revoked_at=? WHERE id=? AND identity_id=?", time.Now().UTC().Format(time.RFC3339Nano), r.PathValue("id"), x.IdentityID)
	httpx.JSON(w, 200, map[string]bool{"revoked": true})
}
func (a *API) onboard(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" {
		httpx.Failure(w, r, 403, "forbidden", "Reader session required")
		return
	}
	var in struct {
		Username, Avatar string
		EmailVisible     bool
	}
	if decode(r, &in) != nil || !auth.ValidUsername(in.Username) {
		httpx.Failure(w, r, 400, "invalid_username", "Use 3 to 24 lowercase letters, digits, or underscores")
		return
	}
	allowed := map[string]bool{"sunrise": true, "hills": true, "ink": true, "agaseke": true, "volcano": true, "coffee": true}
	if !allowed[in.Avatar] {
		httpx.Failure(w, r, 400, "invalid_avatar", "Choose a built-in avatar")
		return
	}
	_, err := a.DB.ExecContext(r.Context(), "INSERT INTO reader_profiles(identity_id,username,avatar_key,email_visible,joined_at)VALUES(?,?,?,?,?)", x.IdentityID, in.Username, in.Avatar, in.EmailVisible, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		httpx.Failure(w, r, 409, "username_unavailable", "That username is unavailable")
		return
	}
	httpx.JSON(w, 201, map[string]string{"username": in.Username})
}
func (a *API) publicPosts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	posts, err := a.Editorial.Latest(r.Context(), limit)
	if err != nil {
		httpx.Failure(w, r, 500, "database_error", "Could not load posts")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=120")
	httpx.JSON(w, 200, posts)
}
func (a *API) publicPost(w http.ResponseWriter, r *http.Request) {
	p, err := a.Editorial.Public(r.Context(), r.PathValue("slug"))
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Failure(w, r, 404, "not_found", "Article not found")
		return
	}
	if err != nil {
		httpx.Failure(w, r, 500, "database_error", "Could not load article")
		return
	}
	httpx.JSON(w, 200, p)
}
func (a *API) createPost(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	var in struct {
		Title, Excerpt string
		Content        json.RawMessage
	}
	if decode(r, &in) != nil {
		httpx.Failure(w, r, 400, "invalid_request", "Invalid post")
		return
	}
	p, err := a.Editorial.Create(r.Context(), x, in.Title, in.Excerpt, in.Content)
	respond(w, r, p, err, 201)
}
func (a *API) getDraft(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	p, err := a.Editorial.GetDraft(r.Context(), x, r.PathValue("id"))
	respond(w, r, p, err, 200)
}
func (a *API) saveDraft(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	var in struct {
		Title, Excerpt string
		Content        json.RawMessage
		Revision       int
	}
	if decode(r, &in) != nil {
		httpx.Failure(w, r, 400, "invalid_request", "Invalid draft")
		return
	}
	p, err := a.Editorial.Save(r.Context(), x, r.PathValue("id"), in.Title, in.Excerpt, in.Content, in.Revision)
	respond(w, r, p, err, 200)
}
func (a *API) checkpoint(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	var in struct{ Reason string }
	_ = decode(r, &in)
	id, err := a.Editorial.Checkpoint(r.Context(), x, r.PathValue("id"), in.Reason)
	respond(w, r, map[string]string{"versionId": id}, err, 201)
}
func (a *API) submit(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	id, err := a.Editorial.Submit(r.Context(), x, r.PathValue("id"))
	respond(w, r, map[string]string{"reviewId": id}, err, 201)
}
func (a *API) publish(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	id, err := a.Editorial.Publish(r.Context(), x, r.PathValue("id"))
	respond(w, r, map[string]string{"versionId": id}, err, 200)
}
func (a *API) fork(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	p, err := a.Editorial.Fork(r.Context(), x, r.PathValue("id"))
	respond(w, r, p, err, 201)
}

func (a *API) versions(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if _, err := a.Editorial.GetDraft(r.Context(), x, r.PathValue("id")); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id,number,title,excerpt,reason,created_by,created_at FROM post_versions WHERE post_id=? ORDER BY number DESC LIMIT 100", r.PathValue("id"))
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, excerpt, reason, by, at string
		var number int
		if err = rows.Scan(&id, &number, &title, &excerpt, &reason, &by, &at); err != nil {
			respond(w, r, nil, err, 0)
			return
		}
		out = append(out, map[string]any{"id": id, "number": number, "title": title, "excerpt": excerpt, "reason": reason, "createdBy": by, "createdAt": at})
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) restoreVersion(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	p, err := a.Editorial.GetDraft(r.Context(), x, r.PathValue("id"))
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	var title, excerpt, content string
	if err = a.DB.QueryRowContext(r.Context(), "SELECT title,excerpt,content_json FROM post_versions WHERE id=? AND post_id=?", r.PathValue("version"), p.ID).Scan(&title, &excerpt, &content); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	saved, err := a.Editorial.Save(r.Context(), x, p.ID, title, excerpt, json.RawMessage(content), p.Revision)
	if err == nil {
		_, err = a.Editorial.Checkpoint(r.Context(), x, p.ID, "restore")
	}
	respond(w, r, saved, err, http.StatusOK)
}

func (a *API) deletePost(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	var in struct{ Title, Confirmation, Reason string }
	if decode(r, &in) != nil || in.Confirmation != "permanently delete" {
		httpx.Failure(w, r, 400, "confirmation_required", "Type the title and permanently delete")
		return
	}
	var title, owner string
	var ever bool
	if err := a.DB.QueryRowContext(r.Context(), "SELECT title,owner_id,ever_published FROM posts WHERE id=?", r.PathValue("id")).Scan(&title, &owner, &ever); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	if in.Title != title || (ever && x.Role != "superadmin") || (!ever && x.Role != "superadmin" && owner != x.IdentityID) {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(r.Context(), "DELETE FROM posts WHERE id=?", r.PathValue("id"))
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO audit_events(actor_id,action,object_type,object_id,detail_json,ip_address,created_at) VALUES(?,'post.deleted','post',?,?,?,?)", x.IdentityID, r.PathValue("id"), `{"reason":`+strconv.Quote(in.Reason)+`}`, auth.ClientIP(r), now)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]bool{"deleted": err == nil}, err, http.StatusOK)
}

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		httpx.JSON(w, 200, []any{})
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT p.id,p.title,p.slug,p.excerpt,p.published_at FROM post_search s JOIN posts p ON p.id=s.post_id WHERE post_search MATCH ? AND p.state='published' ORDER BY rank LIMIT 30", q)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, slug, excerpt string
		var at *string
		if err = rows.Scan(&id, &title, &slug, &excerpt, &at); err != nil {
			respond(w, r, nil, err, 0)
			return
		}
		out = append(out, map[string]any{"id": id, "title": title, "slug": slug, "excerpt": excerpt, "publishedAt": at})
	}
	httpx.JSON(w, 200, out)
}
func (a *API) review(w http.ResponseWriter, r *http.Request, x auth.Actor, approve bool) {
	var in struct{ Reason string }
	_ = decode(r, &in)
	err := a.Editorial.Decide(r.Context(), x, r.PathValue("id"), approve, in.Reason)
	respond(w, r, map[string]bool{"approved": approve}, err, 200)
}
func respond(w http.ResponseWriter, r *http.Request, data any, err error, status int) {
	if err == nil {
		httpx.JSON(w, status, data)
		return
	}
	code := 500
	if errors.Is(err, auth.ErrUnauthorized) {
		code = 403
	} else if errors.Is(err, sql.ErrNoRows) {
		code = 404
	} else if strings.Contains(err.Error(), "conflict") {
		code = 409
	} else {
		code = 400
	}
	httpx.Failure(w, r, code, "request_failed", err.Error())
}
func (a *API) bookmark(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	_, err := a.DB.ExecContext(r.Context(), "INSERT OR IGNORE INTO bookmarks(reader_id,post_id,created_at)VALUES(?,?,?)", x.IdentityID, r.PathValue("id"), time.Now().UTC().Format(time.RFC3339Nano))
	respond(w, r, map[string]bool{"bookmarked": true}, err, 200)
}
func (a *API) unbookmark(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	_, err := a.DB.ExecContext(r.Context(), "DELETE FROM bookmarks WHERE reader_id=? AND post_id=?", x.IdentityID, r.PathValue("id"))
	respond(w, r, map[string]bool{"bookmarked": false}, err, 200)
}
func (a *API) bookmarks(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	rows, err := a.DB.QueryContext(r.Context(), "SELECT p.id,p.title,p.slug,p.excerpt FROM bookmarks b JOIN posts p ON p.id=b.post_id WHERE b.reader_id=? ORDER BY b.created_at DESC", x.IdentityID)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, title, slug, excerpt string
		rows.Scan(&id, &title, &slug, &excerpt)
		out = append(out, map[string]string{"id": id, "title": title, "slug": slug, "excerpt": excerpt})
	}
	httpx.JSON(w, 200, out)
}
func (a *API) comments(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.QueryContext(r.Context(), "SELECT c.id,COALESCE(r.username,'deleted_reader'),COALESCE(v.body,'[deleted]'),c.parent_id,c.depth,c.created_at FROM comments c LEFT JOIN reader_profiles r ON r.identity_id=c.reader_id LEFT JOIN comment_versions v ON v.id=c.public_version_id WHERE c.post_id=? AND c.status='approved' ORDER BY c.created_at", r.PathValue("id"))
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, user, body, created string
		var parent *string
		var depth int
		rows.Scan(&id, &user, &body, &parent, &depth, &created)
		out = append(out, map[string]any{"id": id, "username": user, "body": body, "parentId": parent, "depth": depth, "createdAt": created})
	}
	httpx.JSON(w, 200, out)
}
func (a *API) addComment(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct {
		Body     string
		ParentID *string
	}
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Body)) < 2 || len(in.Body) > 3000 {
		httpx.Failure(w, r, 400, "invalid_comment", "Comment must be 2 to 3000 characters")
		return
	}
	depth := 0
	if in.ParentID != nil {
		var status string
		if a.DB.QueryRowContext(r.Context(), "SELECT depth+1,status FROM comments WHERE id=?", *in.ParentID).Scan(&depth, &status) != nil || status != "approved" || depth > 2 {
			httpx.Failure(w, r, 400, "invalid_parent", "Replies require an approved parent within three levels")
			return
		}
	}
	id := newID()
	version := newID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, _ := a.DB.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	_, err := tx.ExecContext(r.Context(), "INSERT INTO comments(id,post_id,reader_id,parent_id,depth,pending_version_id,status,created_at,updated_at)VALUES(?,?,?,?,?,?,'pending',?,?)", id, r.PathValue("id"), x.IdentityID, in.ParentID, depth, version, now, now)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO comment_versions(id,comment_id,body,created_by,created_at)VALUES(?,?,?,?,?)", version, id, strings.TrimSpace(in.Body), x.IdentityID, now)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]string{"id": id, "status": "pending"}, err, 201)
}
func (a *API) moderate(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Role != "superadmin" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	action := r.PathValue("action")
	if action != "approve" && action != "reject" && action != "hide" && action != "delete" {
		http.NotFound(w, r)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	query := "UPDATE comments SET status=?,updated_at=? WHERE id=?"
	status := action
	if action == "approve" {
		query = "UPDATE comments SET status='approved',public_version_id=pending_version_id,pending_version_id=NULL,updated_at=? WHERE id=?"
		_, err := a.DB.ExecContext(r.Context(), query, now, r.PathValue("id"))
		respond(w, r, map[string]string{"status": "approved"}, err, 200)
		return
	}
	_, err := a.DB.ExecContext(r.Context(), query, status, now, r.PathValue("id"))
	respond(w, r, map[string]string{"status": status}, err, 200)
}
func (a *API) staff(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Role != "superadmin" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT i.id,i.email,s.handle,s.display_name,s.role,s.publish_mode,s.status FROM identities i JOIN staff_profiles s ON s.identity_id=i.id ORDER BY s.created_at")
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, email, handle, name, role, mode, status string
		rows.Scan(&id, &email, &handle, &name, &role, &mode, &status)
		out = append(out, map[string]string{"id": id, "email": email, "handle": handle, "displayName": name, "role": role, "publishMode": mode, "status": status})
	}
	httpx.JSON(w, 200, out)
}
func (a *API) addStaff(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Role != "superadmin" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct{ Email, Handle, DisplayName, Role, PublishMode string }
	if decode(r, &in) != nil || (in.Role != "author" && in.Role != "superadmin") || (in.PublishMode != "direct_publish" && in.PublishMode != "review_required") {
		httpx.Failure(w, r, 400, "invalid_staff", "Invalid staff profile")
		return
	}
	email, err := auth.NormalizeEmail(in.Email)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	id := newID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, _ := a.DB.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), "INSERT INTO identities(id,email,created_at,updated_at)VALUES(?,?,?,?)", id, email, now, now)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,created_at,updated_at)VALUES(?,?,?,?,?,?,?)", id, in.Handle, in.DisplayName, in.Role, in.PublishMode, now, now)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]string{"id": id}, err, 201)
}
func (a *API) updateStaff(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Role != "superadmin" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct{ Role, PublishMode, Status, ReassignTo string }
	if decode(r, &in) != nil || (in.Role != "author" && in.Role != "superadmin") || (in.PublishMode != "direct_publish" && in.PublishMode != "review_required") || (in.Status != "active" && in.Status != "inactive") {
		httpx.Failure(w, r, 400, "invalid_staff", "Invalid staff update")
		return
	}
	id := r.PathValue("id")
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	var oldRole, oldStatus string
	if err = tx.QueryRowContext(r.Context(), "SELECT role,status FROM staff_profiles WHERE identity_id=?", id).Scan(&oldRole, &oldStatus); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	if oldRole == "superadmin" && oldStatus == "active" && (in.Role != "superadmin" || in.Status != "active") {
		var n int
		tx.QueryRowContext(r.Context(), "SELECT count(*) FROM staff_profiles WHERE role='superadmin' AND status='active'").Scan(&n)
		if n <= 1 {
			httpx.Failure(w, r, 409, "last_superadmin", "The last active superadmin cannot be removed")
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(r.Context(), "UPDATE staff_profiles SET role=?,publish_mode=?,status=?,updated_at=? WHERE identity_id=?", in.Role, in.PublishMode, in.Status, now, id)
	if err == nil && in.Status == "inactive" {
		_, err = tx.ExecContext(r.Context(), "UPDATE sessions SET revoked_at=? WHERE identity_id=? AND kind='staff' AND revoked_at IS NULL", now, id)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), "UPDATE posts SET state='frozen' WHERE owner_id=? AND state IN('draft','in_review')", id)
		}
		if err == nil && in.ReassignTo != "" {
			_, err = tx.ExecContext(r.Context(), "UPDATE posts SET owner_id=?,state=CASE WHEN state='frozen' THEN 'draft' ELSE state END WHERE owner_id=? AND ever_published=0", in.ReassignTo, id)
		}
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO audit_events(actor_id,action,object_type,object_id,detail_json,ip_address,created_at)VALUES(?,'staff.updated','staff',?,?,?,?)", x.IdentityID, id, `{"role":"`+in.Role+`","status":"`+in.Status+`"}`, auth.ClientIP(r), now)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]bool{"updated": err == nil}, err, 200)
}
func newID() string { return uuid.NewString() }

func (a *API) uploadMedia(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "staff" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.Failure(w, r, 413, "image_too_large", "Images may be at most 15 MiB")
		return
	}
	alt := strings.TrimSpace(r.FormValue("alt"))
	if alt == "" {
		httpx.Failure(w, r, 400, "alt_required", "Alternative text is required")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Failure(w, r, 400, "file_required", "JPEG or PNG file required")
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 15<<20+1))
	if err != nil || len(body) > 15<<20 {
		httpx.Failure(w, r, 413, "image_too_large", "Images may be at most 15 MiB")
		return
	}
	mime := http.DetectContentType(body)
	if mime != "image/jpeg" && mime != "image/png" {
		httpx.Failure(w, r, 415, "unsupported_image", "Only JPEG and PNG images are accepted")
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 30_000_000 {
		httpx.Failure(w, r, 400, "invalid_image", "Image is malformed or exceeds 30 megapixels")
		return
	}
	img, decoded, err := image.Decode(bytes.NewReader(body))
	if err != nil || decoded != format {
		httpx.Failure(w, r, 400, "invalid_image", "Image could not be decoded safely")
		return
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(a.MediaDir, hash)
	if err = os.MkdirAll(dir, 0o750); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	for name, maxW := range map[string]int{"original": cfg.Width, "large": 1600, "medium": 900, "small": 480} {
		width, height := cfg.Width, cfg.Height
		if width > maxW {
			height = height * maxW / width
			width = maxW
		}
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		tmp, err := os.CreateTemp(dir, "upload-*.tmp")
		if err != nil {
			respond(w, r, nil, err, 0)
			return
		}
		err = jpeg.Encode(tmp, dst, &jpeg.Options{Quality: 86})
		tmp.Close()
		if err == nil {
			err = os.Rename(tmp.Name(), filepath.Join(dir, name+".jpg"))
		}
		if err != nil {
			os.Remove(tmp.Name())
			respond(w, r, nil, err, 0)
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.NewString()
	_, err = a.DB.ExecContext(r.Context(), "INSERT INTO media_assets(id,owner_id,content_hash,mime_type,width,height,bytes,alt_text,caption,credit,created_at)VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(content_hash) DO NOTHING", id, x.IdentityID, hash, mime, cfg.Width, cfg.Height, len(body), alt, r.FormValue("caption"), r.FormValue("credit"), now)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	var actual string
	a.DB.QueryRowContext(r.Context(), "SELECT id FROM media_assets WHERE content_hash=?", hash).Scan(&actual)
	httpx.JSON(w, 201, map[string]any{"id": actual, "hash": hash, "src": "/media/" + hash + "/large.jpg", "width": cfg.Width, "height": cfg.Height, "alt": alt})
}
func (a *API) serveMedia(w http.ResponseWriter, r *http.Request) {
	hash, size := r.PathValue("hash"), strings.TrimSuffix(r.PathValue("file"), ".jpg")
	if !strings.HasSuffix(r.PathValue("file"), ".jpg") {
		http.NotFound(w, r)
		return
	}
	if len(hash) != 64 || !map[string]bool{"original": true, "large": true, "medium": true, "small": true}[size] {
		http.NotFound(w, r)
		return
	}
	var status string
	if a.DB.QueryRowContext(r.Context(), "SELECT status FROM media_assets WHERE content_hash=?", hash).Scan(&status) != nil || status != "public" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, filepath.Join(a.MediaDir, hash, size+".jpg"))
}
