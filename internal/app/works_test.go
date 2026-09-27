package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func saveWork(t *testing.T, s *Store, w Work) Work {
	t.Helper()
	b, e := s.WriteWork(context.Background(), w.ID, ID(), ID(), "save", w)
	if e != nil {
		t.Fatal(e)
	}
	var result Work
	if e = json.Unmarshal(b, &result); e != nil {
		t.Fatal(e)
	}
	return result
}
func workError(t *testing.T, e error, code string) {
	t.Helper()
	var p *Problem
	if !errors.As(e, &p) || p.Code != code {
		t.Fatalf("wanted %s, got %v", code, e)
	}
}
func TestWorkLifecycleAndProvenance(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f := save(t, s, example())
	w := Work{Name: "亚麻衬衫", Category: "上衣", CompletedDate: "2026-09-27", Tags: []string{"秋天", "秋天"}, Fabrics: []WorkFabric{{FabricID: f.ID, Name: "forged", Note: "主布 1.5 米"}}}
	op, fp := ID(), ID()
	b, e := s.WriteWork(ctx, "", op, fp, "save", w)
	if e != nil {
		t.Fatal(e)
	}
	var first Work
	json.Unmarshal(b, &first)
	if first.Revision != 1 || first.Fabrics[0].Name != f.Name || !first.Fabrics[0].Available || len(first.Tags) != 1 {
		t.Fatal(first)
	}
	replay, e := s.WriteWork(ctx, "", op, fp, "save", w)
	if e != nil || !bytes.Equal(b, replay) {
		t.Fatal("create not idempotent", e)
	}
	_, e = s.WriteWork(ctx, "", op, ID(), "save", w)
	workError(t, e, "key_reused")
	fresh, e := s.Get(ctx, f.ID)
	if e != nil || !reflect.DeepEqual(fresh, f) {
		t.Fatal("work changed inventory", e)
	}
	updated := first
	updated.Notes = "袖长缩短 2 cm"
	updated = saveWork(t, s, updated)
	_, e = s.WriteWork(ctx, first.ID, ID(), ID(), "save", first)
	workError(t, e, "revision_conflict")
	if _, e = s.Write(ctx, f.ID, ID(), ID(), "delete", f); e != nil {
		t.Fatal(e)
	}
	updated, e = s.GetWork(ctx, first.ID)
	if e != nil || updated.Fabrics[0].Available || updated.Fabrics[0].Name != f.Name {
		t.Fatal("lost deleted fabric provenance", e)
	}
	// A stale client cannot invent a new association to a deleted fabric.
	_, e = s.WriteWork(ctx, "", ID(), ID(), "save", w)
	workError(t, e, "validation")
	if _, e = s.DB.Exec("DELETE FROM fabrics WHERE id=?", f.ID); e != nil {
		t.Fatal(e)
	}
	updated.Fabrics[0].Name = "forged again"
	updated = saveWork(t, s, updated)
	if updated.Fabrics[0].Name != f.Name {
		t.Fatal("snapshot can be forged")
	}
	list, e := s.ListWorks(ctx, url.Values{"fabric": {f.ID}}, false)
	if e != nil || list.Total != 1 {
		t.Fatal("reverse association lost", e)
	}
	b, e = s.WriteWork(ctx, updated.ID, ID(), ID(), "delete", updated)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(b, &updated)
	list, _ = s.ListWorks(ctx, url.Values{}, false)
	if list.Total != 0 {
		t.Fatal("trash leaked")
	}
	list, _ = s.ListWorks(ctx, url.Values{"trash": {"1"}}, false)
	if list.Total != 1 {
		t.Fatal("trash missing")
	}
	_, e = s.WriteWork(ctx, updated.ID, ID(), ID(), "save", updated)
	workError(t, e, "deleted")
	b, e = s.WriteWork(ctx, updated.ID, ID(), ID(), "restore", updated)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(b, &updated)
	if updated.DeletedAt != nil || updated.Revision != 5 {
		t.Fatal(updated)
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM work_changes WHERE work_id=?", updated.ID).Scan(&count)
	if count != 5 {
		t.Fatal("history missing", count)
	}
}

func workPhoto(t *testing.T, s *Store) Media {
	t.Helper()
	var b bytes.Buffer
	if e := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 24, 32)), nil); e != nil {
		t.Fatal(e)
	}
	m, e := s.Upload(context.Background(), ID(), &b)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestWorkPhotosBackupCleanupAndRollback(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := workPhoto(t, s)
	w := saveWork(t, s, Work{Name: "照片测试", PhotoIDs: []string{m.ID}})
	_, e := s.WriteWork(ctx, "", ID(), ID(), "save", Work{Name: "steal", PhotoIDs: []string{m.ID}})
	workError(t, e, "validation")
	f := example()
	f.PhotoIDs = []string{m.ID}
	_, e = s.Write(ctx, "", ID(), ID(), "save", f)
	workError(t, e, "validation")
	m2 := workPhoto(t, s)
	f.PhotoIDs = []string{m2.ID}
	f = save(t, s, f)
	_, e = s.WriteWork(ctx, "", ID(), ID(), "save", Work{Name: "steal fabric", PhotoIDs: []string{m2.ID}})
	workError(t, e, "validation")
	// The DB check also prevents an older program assigning the same image.
	if _, e = s.DB.Exec("UPDATE media SET fabric_id=? WHERE id=?", f.ID, m.ID); e == nil {
		t.Fatal("dual photo owner accepted")
	}
	stale := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano)
	s.DB.Exec("UPDATE media SET created_at=? WHERE id=?", stale, m.ID)
	// Emulate the previous release's cleanup statement.
	if _, e = s.DB.Exec("DELETE FROM media WHERE fabric_id IS NULL AND created_at<?", time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.MediaPath(ctx, m.ID, "main"); e != nil {
		t.Fatal("legacy cleanup deleted work photo", e)
	}
	if e = s.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	b, e := s.WriteWork(ctx, w.ID, ID(), ID(), "delete", w)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(b, &w)
	_, e = s.MediaPath(ctx, m.ID, "main")
	workError(t, e, "not_found")
	backup, restored := filepath.Join(t.TempDir(), "backup"), filepath.Join(t.TempDir(), "restore")
	if e = s.Backup(ctx, backup); e != nil {
		t.Fatal(e)
	}
	manifest, e := VerifyBackup(backup)
	if e != nil || manifest.Files["media/"+m.ID+".jpg"] == "" {
		t.Fatal("backup lost work photo", e)
	}
	if e = Restore(ctx, backup, restored); e != nil {
		t.Fatal(e)
	}
	r, e := Open(restored, false)
	if e != nil {
		t.Fatal(e)
	}
	defer r.DB.Close()
	if e = r.Check(ctx); e != nil {
		t.Fatal(e)
	}
	b, e = r.WriteWork(ctx, w.ID, ID(), ID(), "restore", w)
	if e != nil {
		t.Fatal(e)
	}
	var rw Work
	json.Unmarshal(b, &rw)
	if len(rw.Photos) != 1 {
		t.Fatal("restore lost photos")
	}
	if _, e = r.MediaPath(ctx, m.ID, "main"); e != nil {
		t.Fatal(e)
	}
	// Removing a photo retains it for 30 days, then cleanup can delete it.
	rw.PhotoIDs = nil
	rw = saveWork(t, r, rw)
	r.DB.Exec("UPDATE media SET removed_at=? WHERE id=?", time.Now().Add(-31*24*time.Hour).UTC().Format(time.RFC3339Nano), m.ID)
	if e = r.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	r.DB.QueryRow("SELECT count(*) FROM media WHERE id=?", m.ID).Scan(&count)
	if count != 0 {
		t.Fatal("removed image never collected")
	}
	// Expired work deletion cascades its photos/history without touching fabric.
	s.DB.Exec("UPDATE works SET deleted_at=? WHERE id=?", time.Now().Add(-31*24*time.Hour).UTC().Format(time.RFC3339Nano), w.ID)
	if e = s.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	s.DB.QueryRow("SELECT count(*) FROM media WHERE id=?", m.ID).Scan(&count)
	if count != 0 {
		t.Fatal("purged work retained media row")
	}
	if _, e = s.Get(ctx, f.ID); e != nil {
		t.Fatal("purge touched fabric", e)
	}
}

func TestWorkMigrationIsExplicitAndAtomic(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f := save(t, s, example())
	_, e := s.DB.Exec("DROP TRIGGER protect_work_media; DROP INDEX media_work; DROP TABLE work_changes; DROP TABLE works; ALTER TABLE media DROP COLUMN work_id;")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Check(ctx); e == nil {
		t.Fatal("missing work schema accepted")
	}
	for range 2 {
		if e = Migrate(ctx, s.Dir); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Check(ctx); e != nil {
		t.Fatal(e)
	}
	after, e := s.Get(ctx, f.ID)
	if e != nil || !reflect.DeepEqual(f, after) {
		t.Fatal("migration changed fabric", e)
	}
	m := workPhoto(t, s)
	_, e = s.DB.Exec("CREATE TRIGGER reject_work_history BEFORE INSERT ON work_changes BEGIN SELECT RAISE(ABORT,'synthetic failure'); END;")
	if e != nil {
		t.Fatal(e)
	}
	op := ID()
	_, e = s.WriteWork(ctx, "", op, ID(), "save", Work{Name: "must roll back", PhotoIDs: []string{m.ID}})
	if e == nil {
		t.Fatal("injected failure ignored")
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM works").Scan(&count)
	if count != 0 {
		t.Fatal("partial work committed")
	}
	s.DB.QueryRow("SELECT count(*) FROM media WHERE id=? AND work_id IS NOT NULL", m.ID).Scan(&count)
	if count != 0 {
		t.Fatal("partial ownership committed")
	}
	s.DB.QueryRow("SELECT count(*) FROM operations WHERE key=?", op).Scan(&count)
	if count != 0 {
		t.Fatal("partial operation committed")
	}
}

func TestWorkValidationSearchAndExport(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, w := range []Work{{}, {Name: "a", CompletedDate: "2026-02-30"}, {Name: "a", Fabrics: []WorkFabric{{FabricID: "bad"}}}, {Name: "a", PhotoIDs: []string{ID(), "bad"}}, {Name: strings.Repeat("长", 201)}, {Name: "a", Notes: strings.Repeat("长", 8001)}} {
		if _, e := s.WriteWork(ctx, "", ID(), ID(), "save", w); e == nil {
			t.Fatal("invalid work accepted", w)
		}
	}
	for range 25 {
		saveWork(t, s, Work{Name: "测试衬衫", Category: "上衣", Pattern: "Pattern No.7", Notes: "调整肩宽"})
	}
	w := saveWork(t, s, Work{Name: "=SUM(1,2)", Category: "包袋", PhotoIDs: []string{workPhoto(t, s).ID}})
	list, e := s.ListWorks(ctx, url.Values{"q": {"pattern no.7"}}, false)
	if e != nil || list.Total != 25 || len(list.Items) != 24 {
		t.Fatal("search/page", e, list.Total)
	}
	page2, e := s.ListWorks(ctx, url.Values{"q": {"肩宽"}, "offset": {"24"}}, false)
	if e != nil || len(page2.Items) != 1 || page2.Items[0].ID == list.Items[0].ID {
		t.Fatal("page 2", e)
	}
	for _, v := range []url.Values{{"sort": {"bad"}}, {"offset": {"-1"}}, {"fabric": {"bad"}}} {
		if _, e = s.ListWorks(ctx, v, false); e == nil {
			t.Fatal("invalid query accepted")
		}
	}
	h := s.Handler(fstest.MapFS{"index.html": {Data: []byte("test")}})
	csv := httptest.NewRecorder()
	h.ServeHTTP(csv, httptest.NewRequest("GET", "/api/works/export?format=csv&category="+url.QueryEscape("包袋"), nil))
	if csv.Code != 200 || !strings.Contains(csv.Body.String(), "'=SUM") || strings.Contains(csv.Body.String(), "测试衬衫") {
		t.Fatal("unsafe/unfiltered csv", csv.Code, csv.Body.String())
	}
	z := httptest.NewRecorder()
	h.ServeHTTP(z, httptest.NewRequest("GET", "/api/works/export?format=zip&category="+url.QueryEscape("包袋"), nil))
	zr, e := zip.NewReader(bytes.NewReader(z.Body.Bytes()), int64(z.Body.Len()))
	if e != nil || len(zr.File) != 2 {
		t.Fatal("zip", e)
	}
	reader, _ := zr.File[0].Open()
	exported, _ := io.ReadAll(reader)
	reader.Close()
	if !bytes.Contains(exported, []byte(w.ID)) {
		t.Fatal("json not exported")
	}
	for _, body := range []string{`{"name":"x","unknown":true}`, `{"name":"x"}{}`} {
		req := httptest.NewRequest("POST", "/api/works", strings.NewReader(body))
		req.Header.Set("Idempotency-Key", ID())
		r := httptest.NewRecorder()
		h.ServeHTTP(r, req)
		if r.Code != 400 {
			t.Fatal("bad JSON accepted", r.Code)
		}
	}
	req := httptest.NewRequest("POST", "/api/works", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	r := httptest.NewRecorder()
	h.ServeHTTP(r, req)
	if r.Code != http.StatusForbidden {
		t.Fatal("cross-site write accepted")
	}
}
