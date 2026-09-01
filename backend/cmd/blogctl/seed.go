package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// seedDemo creates a substantial, deterministic local newsroom dataset. It is
// intentionally made of ordinary SQL records so it also exercises the public
// pages, search index, moderation queues, reader profiles, and bookmarks.
func seedDemo(db *sql.DB) {
	tx, err := db.Begin()
	fatal(err)
	defer tx.Rollback()
	now := time.Now().UTC()
	stamp := func(daysAgo int, hour int) string {
		return now.AddDate(0, 0, -daysAgo).Truncate(time.Minute).Add(time.Duration(hour) * time.Hour).Format(time.RFC3339Nano)
	}

	staff := []struct{ id, email, handle, name, bio, mode string }{
		{"seed-editor-01", "editor@rwandafreespace.test", "mugisha", "Aline Mugisha", "Product designer and relentless tester of everyday digital services.", "direct_publish"},
		{"seed-editor-02", "research@rwandafreespace.test", "uwase", "Patrick Uwase", "Researcher writing about infrastructure, access, and the people behind the interface.", "review_required"},
		{"seed-editor-03", "fieldnotes@rwandafreespace.test", "ndayisenga", "Eric Ndayisenga", "A developer documenting the small frictions that become big barriers.", "direct_publish"},
	}
	for _, s := range staff {
		seedIdentity(tx, s.id, s.email, stamp(180, 8))
		_, err = tx.Exec(`INSERT OR IGNORE INTO staff_profiles(identity_id,handle,display_name,bio,role,publish_mode,status,created_at,updated_at) VALUES(?,?,?,?, 'author',?,'active',?,?)`, s.id, s.handle, s.name, s.bio, s.mode, stamp(180, 8), stamp(2, 9))
		fatal(err)
	}

	categories := []struct{ id, name, slug, description string }{
		{"cat-infra", "Infrastructure", "infrastructure", "Connectivity, reliability, and the systems underneath our screens"},
		{"cat-rights", "Digital rights", "digital-rights", "Privacy, identity, safety, and who gets a say"},
	}
	for _, c := range categories {
		_, err = tx.Exec(`INSERT OR IGNORE INTO categories(id,name,slug,description) VALUES(?,?,?,?)`, c.id, c.name, c.slug, c.description)
		fatal(err)
	}
	tags := []string{"mobile-money", "fintech", "public-services", "accessibility", "internet", "trust", "design-systems", "privacy", "agriculture", "education", "transport", "small-business", "ai", "open-data", "customer-support"}
	for i, name := range tags {
		_, err = tx.Exec(`INSERT OR IGNORE INTO tags(id,name,slug) VALUES(?,?,?)`, fmt.Sprintf("seed-tag-%02d", i+1), name, strings.ReplaceAll(name, " ", "-"))
		fatal(err)
	}

	posts := []struct {
		id, owner, title, slug, excerpt, category, state string
		days                                             int
	}{
		{"seed-post-01", staff[0].id, "The queue is the product: what Kigali’s busiest services get wrong", "the-queue-is-the-product", "A calm interface cannot hide a three-hour wait. We look at how digital queues shape trust in public services.", "cat-public", "published", 3},
		{"seed-post-02", staff[1].id, "A mobile-money receipt should answer one question immediately", "mobile-money-receipt-should-answer-one-question", "When money moves fast, confirmation needs to be clearer than the transaction itself.", "cat-product", "published", 7},
		{"seed-post-03", staff[2].id, "The invisible tax of a website that only works on fast Wi-Fi", "invisible-tax-fast-wifi", "A slow connection is not an edge case in Rwanda. It is a design constraint.", "cat-access", "published", 11},
		{"seed-post-04", staff[0].id, "Why ‘contact us’ is not a customer-support strategy", "why-contact-us-is-not-support", "A phone number at the bottom of a page is not the same thing as a way out.", "cat-product", "published", 16},
		{"seed-post-05", staff[1].id, "Designing a farmer portal for the moments between the fields", "farmer-portal-between-fields", "Agricultural tools have to respect interruptions, shared phones, and seasonal attention.", "cat-public", "published", 22},
		{"seed-post-06", staff[2].id, "The case for boring, local-first government software", "case-for-boring-local-first-government-software", "Reliable public technology should be easy to maintain, easy to explain, and unafraid of being boring.", "cat-infra", "published", 29},
		{"seed-post-07", staff[0].id, "A passwordless login is still a bad login if it strands people", "passwordless-login-strands-people", "OTP flows remove one barrier and often add three others. Here is the checklist we would use.", "cat-rights", "published", 35},
		{"seed-post-08", staff[1].id, "What an accessible campus app would notice first", "accessible-campus-app-notice-first", "Accessibility is not a final audit; it is a map of the assumptions a product makes.", "cat-access", "published", 42},
		{"seed-post-09", staff[2].id, "Open data is useful only after someone can find it", "open-data-after-someone-can-find-it", "A spreadsheet hidden behind a PDF is technically published and practically missing.", "cat-public", "published", 49},
		{"seed-post-10", staff[0].id, "AI chatbots need an honest ‘I don’t know’", "ai-chatbots-need-honest-i-dont-know", "The fastest route to trust is a clear boundary around what an assistant cannot verify.", "cat-rights", "published", 56},
		{"seed-post-11", staff[1].id, "The checkout screen is where small businesses lose people", "checkout-screen-small-businesses", "A local shop does not need more features; it needs fewer abandoned payments.", "cat-product", "published", 64},
		{"seed-post-12", staff[2].id, "A bus timetable is a promise, not decoration", "bus-timetable-is-a-promise", "Transport information earns its keep when it remains useful after the first missed bus.", "cat-public", "published", 73},
		{"seed-post-13", staff[0].id, "The language switcher that changes less than it promises", "language-switcher-changes-less", "Translation is not complete when the navigation changes and the instructions do not.", "cat-access", "published", 81},
		{"seed-post-14", staff[1].id, "Can a digital ID be both convenient and forgetful?", "digital-id-convenient-and-forgetful", "Convenience should not require a permanent record of every place a person has been.", "cat-rights", "published", 90},
		{"seed-post-15", staff[2].id, "The little offline indicator that could save a whole afternoon", "little-offline-indicator", "Good offline states explain what happened, what was saved, and what to try next.", "cat-infra", "published", 98},
		{"seed-post-16", staff[1].id, "A field guide to respectful product research in Rwanda", "field-guide-respectful-product-research", "The next generation of local products deserves research that is local in method, not just in market.", "cat-product", "in_review", 2},
		{"seed-post-17", staff[2].id, "Five fixes for the national services homepage", "five-fixes-national-services-homepage", "A practical redesign exercise for a homepage that has to serve everyone.", "cat-public", "draft", 1},
		{"seed-post-18", staff[0].id, "The community notes experiment", "community-notes-experiment", "A working notebook on how readers can add context without turning critique into a pile-on.", "cat-rights", "in_review", 4},
		{"seed-post-19", staff[0].id, "When a loading spinner becomes a brand impression", "when-loading-spinner-becomes-brand", "Reliability is part of the visual identity, whether a team designs it or not.", "cat-infra", "draft", 9},
		{"seed-post-20", staff[2].id, "A quiet audit of three school payment flows", "quiet-audit-school-payment-flows", "The parent experience starts long before the payment confirmation page.", "cat-ux", "frozen", 120},
	}
	for i, p := range posts {
		created := stamp(p.days+20, 7)
		updated := stamp(p.days, 10)
		_, err = tx.Exec(`INSERT OR IGNORE INTO posts(id,owner_id,title,slug,excerpt,category_id,state,ever_published,created_at,updated_at,published_at) VALUES(?,?,?,?,?,?,?, ?,?,?,CASE WHEN ?='published' THEN ? ELSE NULL END)`, p.id, p.owner, p.title, p.slug, p.excerpt, p.category, p.state, boolInt(p.state == "published"), created, updated, p.state, updated)
		fatal(err)
		body := demoDocument(p.title, i)
		_, err = tx.Exec(`INSERT OR IGNORE INTO post_drafts(post_id,content_json,revision,updated_at) VALUES(?,?,?,?)`, p.id, body, 2+i%4, updated)
		fatal(err)
		version := fmt.Sprintf("seed-version-%02d", i+1)
		_, err = tx.Exec(`INSERT OR IGNORE INTO post_versions(id,post_id,number,content_json,title,excerpt,reason,created_by,created_at) VALUES(?,?,?,?,?,?,?, ?,?)`, version, p.id, 1, body, p.title, p.excerpt, "seeded demo publication", p.owner, updated)
		fatal(err)
		if p.state == "published" {
			_, err = tx.Exec(`UPDATE posts SET published_version_id=?,state='published',ever_published=1,published_at=? WHERE id=?`, version, updated, p.id)
			fatal(err)
		}
		if p.state == "in_review" {
			decision := fmt.Sprintf("seed-review-%02d", i+1)
			_, err = tx.Exec(`INSERT OR IGNORE INTO review_decisions(id,post_id,version_id,submitted_by,status,reason,created_at) VALUES(?,?,?,?, 'pending','Please review the evidence and proposed fix.',?)`, decision, p.id, version, p.owner, updated)
			fatal(err)
		}
		for j := 0; j < 2+(i%3); j++ {
			_, err = tx.Exec(`INSERT OR IGNORE INTO post_tags(post_id,tag_id) VALUES(?,?)`, p.id, fmt.Sprintf("seed-tag-%02d", (i+j)%len(tags)+1))
			fatal(err)
		}
		if p.state == "published" {
			_, err = tx.Exec(`INSERT OR IGNORE INTO post_search(post_id,title,excerpt,body) VALUES(?,?,?,?)`, p.id, p.title, p.excerpt, body)
			fatal(err)
		}
	}

	readers := []struct{ id, email, username, avatar string }{
		{"seed-reader-01", "ineza@readers.rwandafreespace.test", "ineza", "sunrise"}, {"seed-reader-02", "kamanzi@readers.rwandafreespace.test", "kamanzi", "hills"}, {"seed-reader-03", "mutoni@readers.rwandafreespace.test", "mutoni", "ink"}, {"seed-reader-04", "samuel@readers.rwandafreespace.test", "samuelk", "agaseke"}, {"seed-reader-05", "clarisse@readers.rwandafreespace.test", "clarisse", "volcano"}, {"seed-reader-06", "david@readers.rwandafreespace.test", "davidk", "coffee"}, {"seed-reader-07", "alice@readers.rwandafreespace.test", "alice_rw", "sunrise"}, {"seed-reader-08", "emmanuel@readers.rwandafreespace.test", "emmanuel", "hills"}, {"seed-reader-09", "nadia@readers.rwandafreespace.test", "nadia", "ink"}, {"seed-reader-10", "olivier@readers.rwandafreespace.test", "olivier", "agaseke"}, {"seed-reader-11", "divine@readers.rwandafreespace.test", "divine", "volcano"}, {"seed-reader-12", "keza@readers.rwandafreespace.test", "keza", "coffee"},
	}
	for i, r := range readers {
		joined := stamp(150-i*9, 18)
		seedIdentity(tx, r.id, r.email, joined)
		_, err = tx.Exec(`INSERT OR IGNORE INTO reader_profiles(identity_id,username,avatar_key,email_visible,joined_at,status) VALUES(?,?,?,?,?,'active')`, r.id, r.username, r.avatar, boolInt(i%4 == 0), joined)
		fatal(err)
	}

	for i := 0; i < 36; i++ {
		post := posts[i%14]
		reader := readers[i%len(readers)]
		id := fmt.Sprintf("seed-comment-%02d", i+1)
		version := fmt.Sprintf("seed-comment-version-%02d", i+1)
		created := stamp((i%45)+1, 8+(i%10))
		status := "approved"
		if i%13 == 0 {
			status = "pending"
		} else if i%17 == 0 {
			status = "rejected"
		} else if i%19 == 0 {
			status = "hidden"
		}
		parent := any(nil)
		depth := 0
		if i > 2 && i%5 == 0 {
			parent = fmt.Sprintf("seed-comment-%02d", i-1)
			depth = 1
		}
		body := commentBody(i, post.title)
		_, err = tx.Exec(`INSERT OR IGNORE INTO comments(id,post_id,reader_id,parent_id,depth,public_version_id,pending_version_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, post.id, reader.id, parent, depth, versionIf(status == "approved", version), versionIf(status != "approved", version), status, created, created)
		fatal(err)
		_, err = tx.Exec(`INSERT OR IGNORE INTO comment_versions(id,comment_id,body,created_by,created_at) VALUES(?,?,?,?,?)`, version, id, body, reader.id, created)
		fatal(err)
	}
	for i := 0; i < 24; i++ {
		_, err = tx.Exec(`INSERT OR IGNORE INTO bookmarks(reader_id,post_id,created_at) VALUES(?,?,?)`, readers[i%len(readers)].id, posts[(i*3)%14].id, stamp(i+1, 20))
		fatal(err)
	}
	// Keep a realistic open report and a resolved report in the moderation history.
	_, err = tx.Exec(`INSERT OR IGNORE INTO comment_reports(id,comment_id,reader_id,reason,status,created_at) VALUES('seed-report-open','seed-comment-07','seed-reader-03','The comment makes a personal claim about a named person.','open',?)`, stamp(2, 13))
	fatal(err)
	_, err = tx.Exec(`INSERT OR IGNORE INTO comment_reports(id,comment_id,reader_id,reason,status,created_at,resolved_at,resolved_by) VALUES('seed-report-resolved','seed-comment-12','seed-reader-04','Duplicate link posted across several threads.','resolved',?,?,?)`, stamp(40, 13), stamp(37, 10), staff[0].id)
	fatal(err)
	_, err = tx.Exec(`INSERT OR IGNORE INTO account_suspensions(id,identity_id,starts_at,ends_at,reason,created_by,created_at) VALUES('seed-suspension-01','seed-reader-11',?,?, 'Repeated promotional comments; suspension used for moderation testing.',?,?)`, stamp(65, 9), stamp(58, 9), staff[0].id, stamp(65, 9))
	fatal(err)
	for i := 0; i < 18; i++ {
		_, err = tx.Exec(`INSERT OR IGNORE INTO audit_events(actor_id,action,object_type,object_id,detail_json,ip_address,created_at) VALUES(?,?,?,?,?,?,?)`, staff[i%len(staff)].id, []string{"post.created", "post.published", "comment.approved", "taxonomy.updated", "reader.reviewed"}[i%5], []string{"post", "post", "comment", "tag", "reader"}[i%5], posts[i%len(posts)].id, `{"source":"seed-demo"}`, fmt.Sprintf("10.0.0.%d", i+10), stamp(i+1, 11))
		fatal(err)
	}
	fatal(tx.Commit())
	fmt.Printf("demo data ready: %d posts, %d readers, 36 comments, %d bookmarks\n", len(posts), len(readers), 24)
}

func seedIdentity(tx *sql.Tx, id, email, created string) {
	_, err := tx.Exec(`INSERT OR IGNORE INTO identities(id,email,created_at,updated_at) VALUES(?,?,?,?)`, id, email, created, created)
	fatal(err)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func versionIf(ok bool, v string) any {
	if ok {
		return v
	}
	return nil
}

func demoDocument(title string, variant int) string {
	nodes := []any{
		map[string]any{"type": "heading", "attrs": map[string]any{"level": 2}, "content": []any{map[string]any{"type": "text", "text": "The friction"}}},
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": fmt.Sprintf("%s starts with a small moment that most product teams would call an edge case. In practice, it is the moment people remember when they decide whether a digital service respects their time.", title)}}},
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "We tested the journey on ordinary Android phones, with an uneven connection and the questions a first-time user is most likely to ask. The interface was not evaluated for polish alone; it was evaluated for recovery."}}},
		map[string]any{"type": "heading", "attrs": map[string]any{"level": 3}, "content": []any{map[string]any{"type": "text", "text": "What would make it better"}}},
		map[string]any{"type": "bulletList", "content": []any{map[string]any{"type": "listItem", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Say what is happening, and how long the next step is expected to take."}}}}}, map[string]any{"type": "listItem", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Preserve the user’s work when a connection drops or a session expires."}}}}}, map[string]any{"type": "listItem", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Give people a human-readable receipt they can share or return to later."}}}}}}},
		map[string]any{"type": "blockquote", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "The best local products make the next step obvious, even when the network does not."}}}}},
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": fmt.Sprintf("This is a working critique, not a verdict. Teams can improve the experience by measuring the failure state as carefully as the happy path. Variant %d of this demo article includes the same editorial structure used by the real writing workspace.", variant+1)}}},
	}
	data, _ := json.Marshal(map[string]any{"type": "doc", "content": nodes})
	return string(data)
}

func commentBody(i int, title string) string {
	comments := []string{"This is exactly the kind of small detail that becomes expensive at scale. The recovery step is the part I would measure first.", "I tried a similar flow last month. The service itself was useful, but I had no idea whether my first attempt had gone through.", "The suggestion about plain-language receipts is strong. A screenshot is often the only record people can share with support.", "Could the team publish the test conditions as well? Network quality changes the experience more than most screenshots show.", "This feels familiar from services I use every week. The product is not missing ambition; it is missing a clear fallback.", "I appreciate that the critique points to a fix. It makes the conversation feel practical instead of performative."}
	return fmt.Sprintf("%s On ‘%s’, I would add that the failure message should be written before the success screen.", comments[i%len(comments)], title)
}
