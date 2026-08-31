package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/platform/httpx"
)

func (a *API) updateReaderProfile(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" || x.Status != "active" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct {
		Avatar       string
		EmailVisible bool
	}
	allowed := map[string]bool{"sunrise": true, "hills": true, "ink": true, "agaseke": true, "volcano": true, "coffee": true}
	if decode(r, &in) != nil || !allowed[in.Avatar] {
		httpx.Failure(w, r, 400, "invalid_profile", "Choose a built-in avatar")
		return
	}
	_, err := a.DB.ExecContext(r.Context(), "UPDATE reader_profiles SET avatar_key=?,email_visible=? WHERE identity_id=?", in.Avatar, in.EmailVisible, x.IdentityID)
	respond(w, r, map[string]bool{"updated": err == nil}, err, 200)
}

func (a *API) readerComments(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" || x.Status != "active" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT c.id,p.title,p.slug,COALESCE(pending.body,public.body,'[deleted]'),c.status,c.created_at FROM comments c JOIN posts p ON p.id=c.post_id LEFT JOIN comment_versions pending ON pending.id=c.pending_version_id LEFT JOIN comment_versions public ON public.id=c.public_version_id WHERE c.reader_id=? ORDER BY c.created_at DESC", x.IdentityID)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, title, slug, body, status, created string
		if err = rows.Scan(&id, &title, &slug, &body, &status, &created); err != nil {
			break
		}
		out = append(out, map[string]string{"id": id, "title": title, "slug": slug, "body": body, "status": status, "createdAt": created})
	}
	respond(w, r, out, err, 200)
}

func (a *API) publicReader(w http.ResponseWriter, r *http.Request) {
	var identity, username, avatar, joined, email string
	var visible bool
	err := a.DB.QueryRowContext(r.Context(), "SELECT r.identity_id,r.username,r.avatar_key,r.joined_at,r.email_visible,CASE WHEN r.email_visible THEN i.email ELSE '' END FROM reader_profiles r JOIN identities i ON i.id=r.identity_id WHERE r.username=? AND r.status='active'", r.PathValue("username")).Scan(&identity, &username, &avatar, &joined, &visible, &email)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	rows, _ := a.DB.QueryContext(r.Context(), "SELECT c.id,v.body,p.title,p.slug,c.created_at FROM comments c JOIN comment_versions v ON v.id=c.public_version_id JOIN posts p ON p.id=c.post_id WHERE c.reader_id=? AND c.status='approved' ORDER BY c.created_at DESC LIMIT 50", identity)
	comments := []map[string]string{}
	if rows != nil { defer rows.Close(); for rows.Next() { var id, body, title, slug, created string; if rows.Scan(&id,&body,&title,&slug,&created)==nil { comments=append(comments,map[string]string{"id":id,"body":body,"postTitle":title,"postSlug":slug,"createdAt":created}) } } }
	httpx.JSON(w, 200, map[string]any{"username": username, "avatar": avatar, "joinedAt": joined, "emailVisible": visible, "email": email, "comments": comments})
}

func (a *API) editComment(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" || x.Status != "active" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct{ Body string }
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Body)) < 2 || len(in.Body) > 3000 {
		httpx.Failure(w, r, 400, "invalid_comment", "Comment must be 2 to 3000 characters")
		return
	}
	var status string
	if err := a.DB.QueryRowContext(r.Context(), "SELECT status FROM comments WHERE id=? AND reader_id=?", r.PathValue("id"), x.IdentityID).Scan(&status); err != nil {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	version := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), "INSERT INTO comment_versions(id,comment_id,body,created_by,created_at)VALUES(?,?,?,?,?)", version, r.PathValue("id"), strings.TrimSpace(in.Body), x.IdentityID, now)
	if err == nil {
		if status == "pending" {
			_, err = tx.ExecContext(r.Context(), "UPDATE comments SET pending_version_id=?,updated_at=? WHERE id=?", version, now, r.PathValue("id"))
		} else if status == "approved" {
			_, err = tx.ExecContext(r.Context(), "UPDATE comments SET pending_version_id=?,updated_at=? WHERE id=?", version, now, r.PathValue("id"))
		} else {
			err = auth.ErrUnauthorized
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, r, map[string]string{"status": "pending"}, err, 200)
}

func (a *API) reportComment(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" || x.Status != "active" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct{ Reason string }
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Reason)) < 3 || len(in.Reason) > 500 {
		httpx.Failure(w, r, 400, "invalid_report", "Give a short report reason")
		return
	}
	_, err := a.DB.ExecContext(r.Context(), "INSERT INTO comment_reports(id,comment_id,reader_id,reason,created_at)VALUES(?,?,?,?,?)", uuid.NewString(), r.PathValue("id"), x.IdentityID, strings.TrimSpace(in.Reason), time.Now().UTC().Format(time.RFC3339Nano))
	respond(w, r, map[string]bool{"reported": err == nil}, err, 201)
}

func (a *API) deleteReaderAccount(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" {
		respond(w, r, nil, auth.ErrUnauthorized, 0)
		return
	}
	var in struct{ Mode, Confirmation string }
	if decode(r, &in) != nil || in.Confirmation != "delete my account" || !map[string]bool{"preserve": true, "tombstone": true}[in.Mode] {
		httpx.Failure(w, r, 400, "confirmation_required", "Choose a deletion mode and type delete my account")
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	defer tx.Rollback()
	if in.Mode == "preserve" {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM comments WHERE reader_id=? AND status IN('pending','rejected')", x.IdentityID)
	}
	if in.Mode == "tombstone" {
		_, err = tx.ExecContext(r.Context(), "UPDATE comment_versions SET body='[deleted]' WHERE comment_id IN(SELECT id FROM comments WHERE reader_id=?)", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE comments SET reader_id=NULL WHERE reader_id=?", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM bookmarks WHERE reader_id=?", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM otp_challenges WHERE identity_kind='reader' AND email=(SELECT email FROM identities WHERE id=?)", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE identity_id=? AND kind='reader'", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM reader_profiles WHERE identity_id=?", x.IdentityID)
	}
	if err == nil {
		var staff int
		_ = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM staff_profiles WHERE identity_id=?", x.IdentityID).Scan(&staff)
		if staff == 0 {
			_, err = tx.ExecContext(r.Context(), "DELETE FROM identities WHERE id=?", x.IdentityID)
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	for _, name := range []string{auth.ReaderCookie, "rfs_reader_session", auth.ReaderCSRFCookie, "rfs_reader_csrf"} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1, Secure: a.Auth.SecureCookies, SameSite: http.SameSiteLaxMode})
	}
	httpx.JSON(w, 200, map[string]bool{"deleted": true})
}
