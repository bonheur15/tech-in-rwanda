package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"rwandafreespace.com/blog/backend/internal/auth"
	"rwandafreespace.com/blog/backend/internal/platform/config"
	"rwandafreespace.com/blog/backend/internal/platform/database"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cfg, err := config.Load()
	fatal(err)
	db, err := database.Open(context.Background(), cfg.DatabasePath)
	fatal(err)
	defer db.Close()
	switch os.Args[1] {
	case "migrate-status":
		status(db)
	case "bootstrap-superadmin":
		if len(os.Args) != 5 {
			fatal(fmt.Errorf("usage: blogctl bootstrap-superadmin EMAIL HANDLE DISPLAY_NAME"))
		}
		bootstrap(db, os.Args[2], os.Args[3], os.Args[4])
	case "recover-account":
		if len(os.Args) != 3 {
			fatal(fmt.Errorf("usage: blogctl recover-account EMAIL"))
		}
		recover(db, os.Args[2])
	case "backup":
		if len(os.Args) != 3 {
			fatal(fmt.Errorf("usage: blogctl backup OUTPUT.tar.gz"))
		}
		backup(db, cfg, os.Args[2])
	case "verify-backup":
		if len(os.Args) != 3 {
			fatal(fmt.Errorf("usage: blogctl verify-backup ARCHIVE.tar.gz"))
		}
		verify(os.Args[2])
	case "media-check":
		mediaCheck(db, cfg.MediaDir)
	default:
		usage()
	}
}
func usage() {
	fmt.Fprintln(os.Stderr, "blogctl: migrate-status | bootstrap-superadmin EMAIL HANDLE DISPLAY_NAME | recover-account EMAIL | backup FILE | verify-backup FILE | media-check")
	os.Exit(2)
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func status(db *sql.DB) {
	rows, err := db.Query("SELECT version,applied_at FROM schema_migrations ORDER BY version")
	fatal(err)
	defer rows.Close()
	for rows.Next() {
		var v, t string
		rows.Scan(&v, &t)
		fmt.Println(v, t)
	}
}
func bootstrap(db *sql.DB, email, handle, name string) {
	email, err := auth.NormalizeEmail(email)
	fatal(err)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := db.Begin()
	fatal(err)
	defer tx.Rollback()
	var id string
	err = tx.QueryRow("SELECT id FROM identities WHERE email=?", email).Scan(&id)
	if err == sql.ErrNoRows {
		id = uuid.NewString()
		_, err = tx.Exec("INSERT INTO identities(id,email,created_at,updated_at)VALUES(?,?,?,?)", id, email, now, now)
	}
	fatal(err)
	_, err = tx.Exec("INSERT INTO staff_profiles(identity_id,handle,display_name,role,publish_mode,status,created_at,updated_at)VALUES(?,?,?,'superadmin','direct_publish','active',?,?) ON CONFLICT(identity_id) DO UPDATE SET role='superadmin',status='active',updated_at=excluded.updated_at", id, handle, name, now, now)
	fatal(err)
	fatal(tx.Commit())
	fmt.Println("superadmin ready:", email)
}
func recover(db *sql.DB, email string) {
	email, err := auth.NormalizeEmail(email)
	fatal(err)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := db.Exec("UPDATE sessions SET revoked_at=? WHERE identity_id=(SELECT id FROM identities WHERE email=?) AND revoked_at IS NULL", now, email)
	fatal(err)
	n, _ := res.RowsAffected()
	fmt.Println("revoked sessions:", n)
}

type manifest struct {
	CreatedAt      string            `json:"createdAt"`
	DatabaseSHA256 string            `json:"databaseSha256"`
	Media          map[string]string `json:"media"`
}

func backup(db *sql.DB, cfg config.Config, out string) {
	tmp, err := os.CreateTemp("", "rfs-backup-*.sqlite3")
	fatal(err)
	tmp.Close()
	defer os.Remove(tmp.Name())
	escaped := strings.ReplaceAll(tmp.Name(), "'", "''")
	_, err = db.Exec("VACUUM INTO '" + escaped + "'")
	fatal(err)
	m := manifest{CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), DatabaseSHA256: hashFile(tmp.Name()), Media: map[string]string{}}
	filepath.Walk(cfg.MediaDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(cfg.MediaDir, path)
			m.Media[filepath.ToSlash(rel)] = hashFile(path)
		}
		return nil
	})
	f, err := os.Create(out)
	fatal(err)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	addFile(tw, "database/blog.sqlite3", tmp.Name())
	for rel := range m.Media {
		addFile(tw, "media/"+rel, filepath.Join(cfg.MediaDir, rel))
	}
	body, _ := json.MarshalIndent(m, "", "  ")
	h := &tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(body)), ModTime: time.Now()}
	fatal(tw.WriteHeader(h))
	_, err = tw.Write(body)
	fatal(err)
	fatal(tw.Close())
	fatal(gz.Close())
	fatal(f.Close())
	fmt.Println("backup written:", out)
}
func verify(path string) {
	f, err := os.Open(path)
	fatal(err)
	gz, err := gzip.NewReader(f)
	fatal(err)
	tr := tar.NewReader(gz)
	var m manifest
	seen := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		fatal(err)
		if h.Name == "manifest.json" {
			fatal(json.NewDecoder(tr).Decode(&m))
			continue
		}
		hash := sha256.New()
		_, err = io.Copy(hash, tr)
		fatal(err)
		seen[h.Name] = hex.EncodeToString(hash.Sum(nil))
	}
	if seen["database/blog.sqlite3"] != m.DatabaseSHA256 {
		fatal(fmt.Errorf("database checksum mismatch"))
	}
	for name, sum := range m.Media {
		if seen["media/"+name] != sum {
			fatal(fmt.Errorf("media checksum mismatch: %s", name))
		}
	}
	fmt.Println("backup verified")
}
func mediaCheck(db *sql.DB, dir string) {
	rows, err := db.Query("SELECT id,content_hash FROM media_assets")
	fatal(err)
	defer rows.Close()
	missing := 0
	for rows.Next() {
		var id, hash string
		rows.Scan(&id, &hash)
		if _, err = os.Stat(filepath.Join(dir, hash)); err != nil {
			fmt.Println("missing", id, hash)
			missing++
		}
	}
	if missing > 0 {
		os.Exit(1)
	}
	fmt.Println("media consistent")
}
func hashFile(path string) string {
	f, err := os.Open(path)
	fatal(err)
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	fatal(err)
	return hex.EncodeToString(h.Sum(nil))
}
func addFile(tw *tar.Writer, name, path string) {
	f, err := os.Open(path)
	fatal(err)
	defer f.Close()
	st, err := f.Stat()
	fatal(err)
	fatal(tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: st.Size(), ModTime: st.ModTime()}))
	_, err = io.Copy(tw, f)
	fatal(err)
}
