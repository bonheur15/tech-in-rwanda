package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/platform/httpx"
)

func (a *API) editComment(w http.ResponseWriter, r *http.Request, x auth.Actor) {
	if x.Kind != "reader" {
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
	if x.Kind != "reader" {
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
	if in.Mode == "tombstone" {
		_, err = tx.ExecContext(r.Context(), "UPDATE comment_versions SET body='[deleted]' WHERE comment_id IN(SELECT id FROM comments WHERE reader_id=?)", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE comments SET reader_id=NULL WHERE reader_id=?", x.IdentityID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM bookmarks WHERE reader_id=?; DELETE FROM otp_challenges WHERE identity_kind='reader' AND email=(SELECT email FROM identities WHERE id=?); DELETE FROM sessions WHERE identity_id=? AND kind='reader'; DELETE FROM reader_profiles WHERE identity_id=?", x.IdentityID, x.IdentityID, x.IdentityID, x.IdentityID)
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
