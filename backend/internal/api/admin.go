package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/editorial"
	"rwandafreespace.com/blog/backend/internal/platform/httpx"
)

func (a *API) me(w http.ResponseWriter, r *http.Request, actor auth.Actor) {
	data := map[string]any{"identityId": actor.IdentityID, "kind": actor.Kind, "role": actor.Role, "publishMode": actor.PublishMode, "status": actor.Status}
	if actor.Kind == "staff" {
		var handle, name, bio string
		if err := a.DB.QueryRowContext(r.Context(), "SELECT handle,display_name,bio FROM staff_profiles WHERE identity_id=?", actor.IdentityID).Scan(&handle, &name, &bio); err == nil {
			data["handle"] = handle
			data["displayName"] = name
			data["bio"] = bio
		}
	} else {
		var username, avatar string
		var visible bool
		if err := a.DB.QueryRowContext(r.Context(), "SELECT username,avatar_key,email_visible FROM reader_profiles WHERE identity_id=?", actor.IdentityID).Scan(&username, &avatar, &visible); err == nil {
			data["username"] = username
			data["avatar"] = avatar
			data["emailVisible"] = visible
		} else {
			data["onboardingRequired"] = true
		}
	}
	httpx.JSON(w, 200, data)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request, actor auth.Actor) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	names := []string{auth.StaffCookie, "rfs_staff_session"}
	csrfNames := []string{auth.StaffCSRFCookie, "rfs_staff_csrf"}
	if actor.Kind == "reader" {
		names = []string{auth.ReaderCookie, "rfs_reader_session"}
		csrfNames = []string{auth.ReaderCSRFCookie, "rfs_reader_csrf"}
	}
	for _, name := range names {
		if c, e := r.Cookie(name); e == nil {
			a.DB.ExecContext(r.Context(), "UPDATE sessions SET revoked_at=? WHERE token_digest=?", now, auth.Digest(a.Auth.SessionPepper, c.Value))
			http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.Auth.SecureCookies, SameSite: http.SameSiteLaxMode})
		}
	}
	for _, name := range csrfNames {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1, Secure: a.Auth.SecureCookies, SameSite: http.SameSiteLaxMode})
	}
	httpx.JSON(w, 200, map[string]bool{"signedOut": true})
}

func requireStaff(w http.ResponseWriter, r *http.Request, x auth.Actor) bool {
	if x.Kind != "staff" || x.Status != "active" {
		httpx.Failure(w, r, 403, "forbidden", "Active staff access is required")
		return false
	}
	return true
}
func requireSuperadmin(w http.ResponseWriter, r *http.Request, x auth.Actor) bool {
	if !requireStaff(w, r, x) || x.Role != "superadmin" {
		httpx.Failure(w, r, 403, "forbidden", "Superadmin access is required")
		return false
	}
	return true
}

func (a *API) adminOverview(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireStaff(w, r, x) {
		return
	}
	ownerClause, args := "", []any{}
	if x.Role != "superadmin" {
		ownerClause = " AND owner_id=?"
		args = append(args, x.IdentityID)
	}
	counts := map[string]int{}
	for key, q := range map[string]string{"drafts": "SELECT count(*) FROM posts WHERE state IN('draft','frozen')" + ownerClause, "published": "SELECT count(*) FROM posts WHERE state='published'" + ownerClause, "reviews": "SELECT count(*) FROM review_decisions WHERE status='pending'", "comments": "SELECT count(*) FROM comments WHERE status='pending'", "media": "SELECT count(*) FROM media_assets WHERE status='draft'"} {
		var n int
		queryArgs := args
		if x.Role != "superadmin" {
			switch key {
			case "reviews": q += " AND submitted_by=?"; queryArgs = []any{x.IdentityID}
			case "media": q += " AND owner_id=?"; queryArgs = []any{x.IdentityID}
			case "comments": counts[key] = 0; continue
			}
		} else if key == "reviews" || key == "comments" || key == "media" {
			queryArgs = nil
		}
		_ = a.DB.QueryRowContext(r.Context(), q, queryArgs...).Scan(&n)
		counts[key] = n
	}
	httpx.JSON(w, 200, counts)
}

func (a *API) adminPosts(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireStaff(w, r, x) {
		return
	}
	query := "SELECT p.id,p.title,p.slug,p.excerpt,p.state,p.updated_at,p.published_at,p.owner_id,s.display_name FROM posts p JOIN staff_profiles s ON s.identity_id=p.owner_id"
	args := []any{}
	if x.Role != "superadmin" {
		query += " WHERE p.owner_id=?"
		args = append(args, x.IdentityID)
	}
	query += " ORDER BY p.updated_at DESC LIMIT 100"
	rows, err := a.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, slug, excerpt, state, updated, owner, name string
		var published *string
		if err = rows.Scan(&id, &title, &slug, &excerpt, &state, &updated, &published, &owner, &name); err != nil {
			respond(w, r, nil, err, 0)
			return
		}
		out = append(out, map[string]any{"id": id, "title": title, "slug": slug, "excerpt": excerpt, "state": state, "updatedAt": updated, "publishedAt": published, "ownerId": owner, "author": name})
	}
	httpx.JSON(w, 200, out)
}

func (a *API) adminReviews(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireStaff(w, r, x) {
		return
	}
	query := "SELECT d.id,d.post_id,d.version_id,d.status,d.created_at,p.title,s.display_name FROM review_decisions d JOIN posts p ON p.id=d.post_id JOIN staff_profiles s ON s.identity_id=d.submitted_by WHERE d.status='pending'"
	args := []any{}
	if x.Role != "superadmin" { query += " AND d.submitted_by=?"; args = append(args, x.IdentityID) }
	query += " ORDER BY d.created_at"
	rows, err := a.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, post, version, status, created, title, author string
		rows.Scan(&id, &post, &version, &status, &created, &title, &author)
		out = append(out, map[string]any{"id": id, "postId": post, "versionId": version, "status": status, "createdAt": created, "title": title, "author": author})
	}
	httpx.JSON(w, 200, out)
}

func (a *API) adminComments(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT c.id,c.post_id,p.title,COALESCE(rp.username,'deleted_reader'),v.body,c.parent_id,c.depth,c.status,c.created_at FROM comments c JOIN posts p ON p.id=c.post_id LEFT JOIN reader_profiles rp ON rp.identity_id=c.reader_id JOIN comment_versions v ON v.id=c.pending_version_id WHERE c.pending_version_id IS NOT NULL ORDER BY c.created_at LIMIT 100")
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, post, title, user, body, status, created string
		var parent *string
		var depth int
		rows.Scan(&id, &post, &title, &user, &body, &parent, &depth, &status, &created)
		out = append(out, map[string]any{"id": id, "postId": post, "postTitle": title, "username": user, "body": body, "parentId": parent, "depth": depth, "status": status, "createdAt": created})
	}
	httpx.JSON(w, 200, out)
}

func (a *API) adminReports(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT cr.id,cr.comment_id,cr.reason,cr.status,cr.created_at,r.username,COALESCE(v.body,'[deleted]') FROM comment_reports cr JOIN reader_profiles r ON r.identity_id=cr.reader_id JOIN comments c ON c.id=cr.comment_id LEFT JOIN comment_versions v ON v.id=c.public_version_id WHERE cr.status='open' ORDER BY cr.created_at")
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, comment, reason, status, created, username, body string
		if err = rows.Scan(&id, &comment, &reason, &status, &created, &username, &body); err != nil {
			break
		}
		out = append(out, map[string]string{"id": id, "commentId": comment, "reason": reason, "status": status, "createdAt": created, "username": username, "body": body})
	}
	respond(w, r, out, err, 200)
}

func (a *API) resolveReport(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := a.DB.ExecContext(r.Context(), "UPDATE comment_reports SET status='resolved',resolved_at=?,resolved_by=? WHERE id=? AND status='open'", now, x.IdentityID, r.PathValue("id"))
	if err == nil {
		if n, _ := result.RowsAffected(); n == 0 {
			err = sql.ErrNoRows
		}
	}
	respond(w, r, map[string]bool{"resolved": err == nil}, err, 200)
}

func (a *API) adminMedia(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireStaff(w, r, x) {
		return
	}
	query := "SELECT id,content_hash,mime_type,width,height,bytes,alt_text,caption,credit,status,created_at,owner_id FROM media_assets"
	args := []any{}
	if x.Role != "superadmin" {
		query += " WHERE owner_id=?"
		args = append(args, x.IdentityID)
	}
	query += " ORDER BY created_at DESC LIMIT 100"
	rows, err := a.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, hash, mime, alt, caption, credit, status, created, owner string
		var width, height, bytes int
		rows.Scan(&id, &hash, &mime, &width, &height, &bytes, &alt, &caption, &credit, &status, &created, &owner)
		out = append(out, map[string]any{"id": id, "hash": hash, "src": "/media/" + hash + "/small.jpg", "mimeType": mime, "width": width, "height": height, "bytes": bytes, "alt": alt, "caption": caption, "credit": credit, "status": status, "createdAt": created, "ownerId": owner})
	}
	httpx.JSON(w, 200, out)
}

func (a *API) adminReaders(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT rp.identity_id,rp.username,rp.avatar_key,rp.email_visible,rp.joined_at,rp.status,CASE WHEN rp.email_visible=1 THEN i.email ELSE '' END,(SELECT count(*) FROM comments c WHERE c.reader_id=rp.identity_id AND c.status='approved') FROM reader_profiles rp JOIN identities i ON i.id=rp.identity_id ORDER BY rp.joined_at DESC LIMIT 100")
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, user, avatar, joined, status, email string
		var visible bool
		var comments int
		rows.Scan(&id, &user, &avatar, &visible, &joined, &status, &email, &comments)
		out = append(out, map[string]any{"id": id, "username": user, "avatar": avatar, "emailVisible": visible, "email": email, "joinedAt": joined, "status": status, "approvedComments": comments})
	}
	httpx.JSON(w, 200, out)
}

func (a *API) adminAudit(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id,COALESCE(actor_id,''),action,object_type,object_id,detail_json,ip_address,created_at FROM audit_events ORDER BY id DESC LIMIT 200")
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var actor, action, typ, obj, detail, ip, created string
		rows.Scan(&id, &actor, &action, &typ, &obj, &detail, &ip, &created)
		out = append(out, map[string]any{"id": id, "actorId": actor, "action": action, "objectType": typ, "objectId": obj, "detail": detail, "ipAddress": ip, "createdAt": created})
	}
	httpx.JSON(w, 200, out)
}

func (a *API) updateReader(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	var in struct {
		Status, Reason string
		DurationDays   int
	}
	if decode(r, &in) != nil || !map[string]bool{"active": true, "suspended": true, "banned": true}[in.Status] {
		httpx.Failure(w, r, 400, "invalid_status", "Choose active, suspended, or banned")
		return
	}
	if in.Status == "suspended" && (in.DurationDays < 1 || in.DurationDays > 365) {
		httpx.Failure(w, r, 400, "invalid_suspension", "Suspensions must last from 1 to 365 days")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), "UPDATE reader_profiles SET status=? WHERE identity_id=?", in.Status, r.PathValue("id"))
	if err == nil && in.Status != "active" {
		_, err = tx.ExecContext(r.Context(), "UPDATE sessions SET revoked_at=? WHERE identity_id=? AND kind='reader' AND revoked_at IS NULL", now, r.PathValue("id"))
	}
	if err == nil && in.Status == "suspended" {
		ends := time.Now().UTC().Add(time.Duration(in.DurationDays) * 24 * time.Hour).Format(time.RFC3339Nano)
		_, err = tx.ExecContext(r.Context(), "INSERT INTO account_suspensions(id,identity_id,starts_at,ends_at,reason,created_by,created_at)VALUES(?,?,?,?,?,?,?)", uuid.NewString(), r.PathValue("id"), now, ends, strings.TrimSpace(in.Reason), x.IdentityID, now)
	}
	if err == nil {
		detail, _ := json.Marshal(map[string]string{"status": in.Status, "reason": in.Reason})
		_, err = tx.ExecContext(r.Context(), "INSERT INTO audit_events(actor_id,action,object_type,object_id,detail_json,ip_address,created_at) VALUES(?,'reader.status_changed','reader',?,?,?,?)", x.IdentityID, r.PathValue("id"), string(detail), auth.ClientIP(r), now)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]bool{"updated": err == nil}, err, 200)
}

func (a *API) updateProfile(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "staff" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct{ DisplayName, Handle, Bio string }
	if decode(r, &in) != nil || strings.TrimSpace(in.DisplayName) == "" || !auth.ValidUsername(in.Handle) {
		httpx.Failure(w, r, 400, "invalid_profile", "Display name and a valid handle are required")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	var old string
	if err = tx.QueryRowContext(r.Context(), "SELECT handle FROM staff_profiles WHERE identity_id=?", x.IdentityID).Scan(&old); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	if old != in.Handle {
		_, err = tx.ExecContext(r.Context(), "INSERT OR IGNORE INTO staff_handle_aliases(handle,identity_id,created_at)VALUES(?,?,?)", old, x.IdentityID, now)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE staff_profiles SET handle=?,display_name=?,bio=?,updated_at=? WHERE identity_id=?", in.Handle, strings.TrimSpace(in.DisplayName), strings.TrimSpace(in.Bio), now, x.IdentityID)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]bool{"updated": err == nil}, err, 200)
}

func (a *API) attachMedia(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireStaff(w, r, x) {
		return
	}
	var in struct{ Placement string }
	if decode(r, &in) != nil || !map[string]bool{"thumbnail": true, "center": true, "wide": true, "full": true, "left": true, "right": true}[in.Placement] {
		httpx.Failure(w, r, 400, "invalid_placement", "Choose a supported image placement")
		return
	}
	p, err := a.Editorial.GetDraft(r.Context(), x, r.PathValue("id"))
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	var owner, status string
	if err = a.DB.QueryRowContext(r.Context(), "SELECT owner_id,status FROM media_assets WHERE id=?", r.PathValue("asset")).Scan(&owner, &status); err != nil || (owner != x.IdentityID && status != "public" && x.Role != "superadmin") {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	if in.Placement == "thumbnail" {
		_, err = tx.ExecContext(r.Context(), "UPDATE posts SET thumbnail_asset_id=?,updated_at=? WHERE id=?", r.PathValue("asset"), time.Now().UTC().Format(time.RFC3339Nano), p.ID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "INSERT OR REPLACE INTO article_media(post_id,asset_id,placement)VALUES(?,?,?)", p.ID, r.PathValue("asset"), in.Placement)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]bool{"attached": err == nil}, err, 200)
}

func (a *API) createCategory(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireSuperadmin(w, r, x) {
		return
	}
	var in struct{ Name, Slug, Description string }
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		httpx.Failure(w, r, 400, "invalid_category", "Category name is required")
		return
	}
	if in.Slug == "" {
		in.Slug = editorial.Slug(in.Name)
	}
	id := uuid.NewString()
	_, err := a.DB.ExecContext(r.Context(), "INSERT INTO categories(id,name,slug,description)VALUES(?,?,?,?)", id, strings.TrimSpace(in.Name), editorial.Slug(in.Slug), strings.TrimSpace(in.Description))
	respond(w, r, map[string]string{"id": id, "slug": editorial.Slug(in.Slug)}, err, 201)
}
func (a *API) createTag(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if !requireStaff(w, r, x) {
		return
	}
	var in struct{ Name, Slug string }
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		httpx.Failure(w, r, 400, "invalid_tag", "Tag name is required")
		return
	}
	if in.Slug == "" {
		in.Slug = editorial.Slug(in.Name)
	}
	id := uuid.NewString()
	_, err := a.DB.ExecContext(r.Context(), "INSERT INTO tags(id,name,slug)VALUES(?,?,?)", id, strings.TrimSpace(in.Name), editorial.Slug(in.Slug))
	respond(w, r, map[string]string{"id": id, "slug": editorial.Slug(in.Slug)}, err, 201)
}
func (a *API) updatePostMetadata(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	p, err := a.Editorial.GetDraft(r.Context(), x, r.PathValue("id"))
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	var in struct {
		CategoryID string
		TagIDs     []string
	}
	if decode(r, &in) != nil {
		httpx.Failure(w, r, 400, "invalid_metadata", "Invalid metadata")
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	var category any = nil
	if in.CategoryID != "" {
		category = in.CategoryID
	}
	_, err = tx.ExecContext(r.Context(), "UPDATE posts SET category_id=?,updated_at=? WHERE id=?", category, time.Now().UTC().Format(time.RFC3339Nano), p.ID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM post_tags WHERE post_id=?", p.ID)
	}
	for _, tag := range in.TagIDs {
		if err != nil {
			break
		}
		_, err = tx.ExecContext(r.Context(), "INSERT INTO post_tags(post_id,tag_id)VALUES(?,?)", p.ID, tag)
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]bool{"updated": err == nil}, err, 200)
}
