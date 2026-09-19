package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(t.TempDir(), true)
	if e != nil {
		t.Fatal(e)
	}
	s.FreeReserve = 0
	t.Cleanup(func() { s.DB.Close() })
	return s
}
func example() Fabric {
	return Fabric{Name: "小花棉麻", Materials: []string{"棉", "麻"}, Status: "unused", Location: "衣柜 / 二号箱", Price: "12.34", Pieces: []Piece{{Width: "150", Length: "250", Unit: "cm", Count: 1}}}
}
func save(t *testing.T, s *Store, f Fabric) Fabric {
	t.Helper()
	b, e := s.Write(context.Background(), f.ID, ID(), ID(), "save", f)
	if e != nil {
		t.Fatal(e)
	}
	var out Fabric
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestDimensionsAndValidation(t *testing.T) {
	f := example()
	f.Pieces[0].Unit = "m"
	f.Pieces[0].Width = "1.5"
	f.Pieces[0].Length = "2.501"
	if e := f.Validate(); e != nil {
		t.Fatal(e)
	}
	if *f.Pieces[0].WidthMM != 1500 || *f.Pieces[0].LengthMM != 2501 || *f.PriceCents != 1234 {
		t.Fatal(f)
	}
	for _, bad := range []string{"-1", "NaN", "1e2", "0", "1.0001", "1001"} {
		f.Pieces[0].Length = bad
		if e := f.Validate(); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	f.Pieces[0].Length = ""
	if e := f.Validate(); e != nil {
		t.Fatal(e)
	}
	if f.Pieces[0].LengthMM != nil {
		t.Fatal("unknown became zero")
	}
	f.Status = "used"
	if f.Validate() == nil {
		t.Fatal("used with stock")
	}
}
func TestIdempotencyAndConcurrentEdits(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	k := ID()
	f := example()
	b, e := s.Write(ctx, "", k, "request", "save", f)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Write(ctx, "", k, "request", "save", f)
	if e != nil || !bytes.Equal(b, again) {
		t.Fatal("replay differs", e)
	}
	if _, e = s.Write(ctx, "", k, "different", "save", f); e == nil {
		t.Fatal("key reused")
	}
	json.Unmarshal(b, &f)
	old := f
	f.Name = "已修改"
	f = save(t, s, f)
	if f.Revision != 2 {
		t.Fatal(f.Revision)
	}
	if _, e = s.Write(ctx, old.ID, ID(), "stale", "save", old); e == nil {
		t.Fatal("stale overwrite")
	}
	list, e := s.List(ctx, url.Values{}, false)
	if e != nil || list.Total != 1 {
		t.Fatal(list, e)
	}
}
func TestStockFilterAndTrash(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a := save(t, s, example())
	b := example()
	b.Name = "不规则棉布"
	b.Pieces[0].Irregular = true
	save(t, s, b)
	list, e := s.List(ctx, url.Values{"q": {"棉"}, "min_width": {"140"}, "min_length": {"200"}}, false)
	if e != nil || list.Total != 1 || list.Items[0].ID != a.ID {
		t.Fatal(list, e)
	}
	a.Pieces = []Piece{}
	a.Status = "used"
	a = save(t, s, a)
	list, e = s.List(ctx, url.Values{"status": {"stock"}}, false)
	if e != nil || list.Total != 1 {
		t.Fatal(list, e)
	}
	result, e := s.Write(ctx, a.ID, ID(), ID(), "delete", a)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(result, &a)
	list, e = s.List(ctx, url.Values{"trash": {"1"}}, false)
	if e != nil || list.Total != 1 {
		t.Fatal(list, e)
	}
	result, e = s.Write(ctx, a.ID, ID(), ID(), "restore", a)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(result, &a)
	if a.DeletedAt != nil {
		t.Fatal("not restored")
	}
	rows, e := s.DB.Query("SELECT body FROM changes WHERE fabric_id=?", a.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	if n != 4 {
		t.Fatal("missing history", n)
	}
}
func TestRemnantPreservesDetailsAndPhotoOwnership(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Media{ID: ID(), Width: 100, Height: 60, Bytes: 1234, CreatedAt: now()}
	metadata, _ := json.Marshal(m)
	if _, e := s.DB.Exec("INSERT INTO media(id,body,created_at) VALUES(?,?,?)", m.ID, string(metadata), m.CreatedAt); e != nil {
		t.Fatal(e)
	}
	f := example()
	f.PhotoIDs = []string{m.ID}
	f.Notes, f.Shop = "保留这段备注", "布料店"
	f = save(t, s, f)
	f.Pieces[0].Length = "240"
	f = save(t, s, f)
	if f.Status != "unused" {
		t.Fatal("a measurement correction consumed fabric")
	}
	in := Fabric{Revision: f.Revision, Pieces: []Piece{{Width: "150", Length: "180", Unit: "cm", Count: 1}}, Status: "unused", Name: "must be ignored", PhotoIDs: []string{ID()}, Price: "999"}
	k := ID()
	body, e := s.Write(ctx, f.ID, k, "remnant-request", "remnant", in)
	if e != nil {
		t.Fatal(e)
	}
	var after Fabric
	if e = json.Unmarshal(body, &after); e != nil {
		t.Fatal(e)
	}
	if after.Status != "using" || after.Revision != f.Revision+1 || len(after.Pieces) != 1 || *after.Pieces[0].LengthMM != 1800 {
		t.Fatal("incorrect stock update", after)
	}
	expected := f
	expected.Status, expected.Pieces = after.Status, after.Pieces
	expected.Revision, expected.UpdatedAt = after.Revision, after.UpdatedAt
	if !reflect.DeepEqual(expected, after) {
		t.Fatal("remnant altered descriptive data or photos")
	}
	var untouched bool
	if e = s.DB.QueryRow("SELECT fabric_id=? AND removed_at IS NULL FROM media WHERE id=?", f.ID, m.ID).Scan(&untouched); e != nil || !untouched {
		t.Fatal("photo association changed", e)
	}
	replayed, e := s.Write(ctx, f.ID, k, "remnant-request", "remnant", in)
	if e != nil || !bytes.Equal(body, replayed) {
		t.Fatal("remnant replay differs", e)
	}
	if _, e = s.Write(ctx, f.ID, ID(), "stale", "remnant", in); e == nil {
		t.Fatal("stale remnant overwrote newer state")
	}
	if _, e = s.Write(ctx, "", ID(), "new", "remnant", in); e == nil {
		t.Fatal("remnant created new fabric")
	}
	in.Revision, in.Status = after.Revision, "used"
	if _, e = s.Write(ctx, f.ID, ID(), "invalid-stock", "remnant", in); e == nil {
		t.Fatal("used status accepted leftover pieces")
	}
	body, e = s.Write(ctx, f.ID, ID(), "used", "remnant", Fabric{Revision: after.Revision, Status: "used", Pieces: []Piece{}})
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(body, &after)
	if after.Status != "used" || len(after.Pieces) != 0 || len(after.Photos) != 1 || after.Name != f.Name {
		t.Fatal("used lost record details")
	}
	var history string
	if e = s.DB.QueryRow("SELECT body FROM changes WHERE fabric_id=? AND revision=?", f.ID, after.Revision).Scan(&history); e != nil {
		t.Fatal(e)
	}
	var change Change
	json.Unmarshal([]byte(history), &change)
	if change.Action != "remnant" || change.Before.Pieces[0].Length != "180" || len(change.After.Pieces) != 0 {
		t.Fatal("missing stock history")
	}
}
func TestUploadBackupRestore(t *testing.T) {
	if _, e := exec.LookPath("vips"); e != nil {
		t.Skip("libvips required")
	}
	s := testStore(t)
	ctx := context.Background()
	img := image.NewRGBA(image.Rect(0, 0, 100, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	k := ID()
	m, e := s.Upload(ctx, k, bytes.NewReader(buf.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	m2, e := s.Upload(ctx, k, bytes.NewReader(buf.Bytes()))
	if e != nil || m.ID != m2.ID {
		t.Fatal("upload replay", e)
	}
	f := example()
	f.PhotoIDs = []string{m.ID}
	f = save(t, s, f)
	if len(f.Photos) != 1 {
		t.Fatal("photo missing")
	}
	another := example()
	another.PhotoIDs = []string{m.ID}
	if _, e = s.Write(ctx, "", ID(), ID(), "save", another); e == nil {
		t.Fatal("photo can be stolen")
	}
	out := filepath.Join(t.TempDir(), "backup")
	if e = s.Backup(ctx, out); e != nil {
		t.Fatal(e)
	}
	if _, e = VerifyBackup(out); e != nil {
		t.Fatal(e)
	}
	dst := filepath.Join(t.TempDir(), "restore")
	if e = Restore(ctx, out, dst); e != nil {
		t.Fatal(e)
	}
	restored, e := Open(dst, false)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.DB.Close()
	rf, e := restored.Get(ctx, f.ID)
	if e != nil || rf.Name != f.Name || len(rf.Photos) != 1 {
		t.Fatal("incomplete restore", e)
	}
	if _, e = s.Upload(ctx, ID(), strings.NewReader("<svg></svg>")); e == nil {
		t.Fatal("SVG accepted")
	}
	if e = Restore(ctx, out, dst); e == nil {
		t.Fatal("overwrite restore")
	}
}
func TestBackupLockAndMissingPhoto(t *testing.T) {
	s := testStore(t)
	unlock, e := s.Lock(false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Cleanup(context.Background()); e == nil {
		t.Fatal("cleanup did not respect lock")
	}
	unlock()
	m := Media{ID: ID(), CreatedAt: now()}
	b, _ := json.Marshal(m)
	s.DB.Exec("INSERT INTO media(id,body,created_at) VALUES(?,?,?)", m.ID, string(b), m.CreatedAt)
	out := filepath.Join(t.TempDir(), "incomplete")
	if e = s.Backup(context.Background(), out); e == nil {
		t.Fatal("accepted missing media")
	}
	if _, e = os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("incomplete backup retained")
	}
}
func TestHTTPBoundaries(t *testing.T) {
	s := testStore(t)
	assets := fstest.MapFS{"index.html": {Data: []byte("test app")}, "assets/test.js": {Data: []byte("true")}}
	server := httptest.NewServer(s.Handler(assets))
	defer server.Close()
	body, _ := json.Marshal(example())
	req, _ := http.NewRequest("POST", server.URL+"/api/fabrics", bytes.NewReader(body))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Idempotency-Key", ID())
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal(res.StatusCode)
	}
	for path, status := range map[string]int{"/healthz": 200, "/fabrics/deep": 200, "/assets/missing.js": 404, "/api/missing": 404} {
		res, e = http.Get(server.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != status {
			t.Fatal(path, res.StatusCode)
		}
	}
	f := example()
	f.Name = "=SUM(A1)"
	save(t, s, f)
	res, e = http.Get(server.URL + "/api/export?format=csv")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(b, []byte("'=SUM(A1)")) {
		t.Fatal("CSV formula not escaped")
	}
}
func TestMissingDatabaseAndCleanup(t *testing.T) {
	if _, e := Open(filepath.Join(t.TempDir(), "missing"), false); e == nil {
		t.Fatal("created empty db")
	}
	s := testStore(t)
	f := save(t, s, example())
	cut := time.Now().Add(-31 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	s.DB.Exec("UPDATE fabrics SET deleted_at=? WHERE id=?", cut, f.ID)
	if e := s.Cleanup(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Get(context.Background(), f.ID); e == nil {
		t.Fatal("expired trash retained")
	}
}
