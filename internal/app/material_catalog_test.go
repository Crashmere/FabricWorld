package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func usage(t *testing.T, s *Store, name string) MaterialUsage {
	t.Helper()
	items, e := s.Materials(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("missing material %s", name)
	return MaterialUsage{}
}

func TestRemoveMaterialAcrossAllFabrics(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f := example()
	f.MaterialPercentages = map[string]string{"棉": "75.5", "麻": "40"}
	f.Composition, f.Notes = "说明里保留棉字", "保留备注"
	m := Media{ID: ID(), Width: 100, Height: 60, Bytes: 1234, CreatedAt: now()}
	metadata, _ := json.Marshal(m)
	if _, e := s.DB.Exec("INSERT INTO media(id,body,created_at) VALUES(?,?,?)", m.ID, string(metadata), m.CreatedAt); e != nil {
		t.Fatal(e)
	}
	f.PhotoIDs = []string{m.ID}
	a := save(t, s, f)
	f.PhotoIDs = nil
	f.Materials = []string{"棉"}
	b := save(t, s, f)
	deleted, e := s.Write(ctx, b.ID, ID(), ID(), "delete", b)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(deleted, &b)
	f.Status, f.Pieces = "used", []Piece{}
	c := save(t, s, f)
	unrelated := example()
	unrelated.Materials = []string{"棉麻"}
	u := save(t, s, unrelated)
	item := usage(t, s, "棉")
	if item.Count != 3 || item.TrashCount != 1 {
		t.Fatal(item)
	}
	k := ID()
	in := RemoveMaterialInput{Name: item.Name, Version: item.Version}
	result, e := s.RemoveMaterial(ctx, k, "remove-cotton", in)
	if e != nil {
		t.Fatal(e)
	}
	var removed RemoveMaterialResult
	json.Unmarshal(result, &removed)
	if removed.Affected != 3 {
		t.Fatal(removed)
	}
	for _, before := range []Fabric{a, b, c} {
		after, e := s.Get(ctx, before.ID)
		if e != nil {
			t.Fatal(e)
		}
		if after.Revision != before.Revision+1 {
			t.Fatal("missing revision")
		}
		expected := before
		expected.Revision, expected.UpdatedAt = after.Revision, after.UpdatedAt
		expected.Materials = []string{}
		expected.MaterialPercentages = nil
		if before.ID == a.ID {
			expected.Materials = []string{"麻"}
			expected.MaterialPercentages = map[string]string{"麻": "100"}
		}
		if !reflect.DeepEqual(after, expected) {
			t.Fatalf("changed unrelated fields: %#v / %#v", after, expected)
		}
		var history string
		if e = s.DB.QueryRow("SELECT body FROM changes WHERE fabric_id=? AND revision=?", after.ID, after.Revision).Scan(&history); e != nil {
			t.Fatal(e)
		}
		var change Change
		json.Unmarshal([]byte(history), &change)
		if change.Action != "remove_material" || change.Before.Materials[0] != "棉" || len(change.After.Materials) == len(change.Before.Materials) {
			t.Fatal(change)
		}
	}
	unchanged, _ := s.Get(ctx, u.ID)
	if !reflect.DeepEqual(unchanged, u) {
		t.Fatal("substring match altered an unrelated material")
	}
	if _, e = s.Write(ctx, a.ID, ID(), ID(), "save", a); e == nil {
		t.Fatal("stale editor resurrected deleted material")
	}
	list, e := s.List(ctx, url.Values{"material": {"棉"}, "status": {"all"}}, false)
	if e != nil || list.Total != 0 {
		t.Fatal(list, e)
	}
	save(t, s, f) // Explicit re-add after removal; an old retry must not remove it.
	replayed, e := s.RemoveMaterial(ctx, k, "remove-cotton", in)
	if e != nil || !bytes.Equal(result, replayed) || usage(t, s, "棉").Count != 1 {
		t.Fatal("retry changed newly added fabric", e)
	}
	if _, e = s.RemoveMaterial(ctx, k, "different-request", in); e == nil {
		t.Fatal("reused operation key")
	}
}

func TestRemoveMaterialConflictAndRollback(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a := save(t, s, example())
	b := save(t, s, example())
	if _, e := s.AddMaterial(ctx, ID(), ID(), AddMaterialInput{Name: "棉"}); e != nil {
		t.Fatal(e)
	}
	item := usage(t, s, "棉")
	a.Notes = "并发编辑"
	a = save(t, s, a)
	if _, e := s.RemoveMaterial(ctx, ID(), ID(), RemoveMaterialInput{Name: item.Name, Version: item.Version}); e == nil {
		t.Fatal("stale count accepted")
	}
	if usage(t, s, "棉").Count != 2 {
		t.Fatal("partial change on conflict")
	}
	item = usage(t, s, "棉")
	// Force a failure after fabric updates to prove the entire transaction rolls back.
	if _, e := s.DB.Exec(`CREATE TRIGGER fail_material_history BEFORE INSERT ON changes
WHEN json_extract(NEW.body,'$.action')='remove_material' BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.RemoveMaterial(ctx, ID(), ID(), RemoveMaterialInput{Name: item.Name, Version: item.Version}); e == nil {
		t.Fatal("injected failure not reached")
	}
	for _, before := range []Fabric{a, b} {
		after, e := s.Get(ctx, before.ID)
		if e != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("batch removal was not atomic", e)
		}
	}
	if usage(t, s, "棉").Version != item.Version {
		t.Fatal("failed removal changed catalog entry")
	}
}

func TestStandaloneMaterialLifecycle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	in := AddMaterialInput{Name: "  新材质  "}
	k := ID()
	result, e := s.AddMaterial(ctx, k, "add", in)
	if e != nil || string(result) != `{"name":"新材质"}` {
		t.Fatal(string(result), e)
	}
	empty := usage(t, s, "新材质")
	if empty.Count != 0 || empty.TrashCount != 0 {
		t.Fatal(empty)
	}
	if _, e = s.AddMaterial(ctx, ID(), "duplicate", in); e != nil || usage(t, s, "新材质") != empty {
		t.Fatal("duplicate changed material identity", e)
	}
	suggestions, e := s.Suggestions(ctx)
	if e != nil || !slices.Contains(suggestions["materials"], "新材质") {
		t.Fatal(suggestions, e)
	}
	f := example()
	f.Materials = []string{"新材质", "麻"}
	f = save(t, s, f)
	if usage(t, s, "新材质").Count != 1 {
		t.Fatal("catalog entry counted as a fabric")
	}
	if _, e = s.RemoveMaterial(ctx, ID(), ID(), RemoveMaterialInput{Name: empty.Name, Version: empty.Version}); e == nil {
		t.Fatal("new usage did not invalidate empty catalog version")
	}
	f.Materials = []string{"麻"}
	save(t, s, f)
	if usage(t, s, "新材质").Count != 0 {
		t.Fatal("standalone material did not survive losing its last usage")
	}
	removeKey := ID()
	removeIn := RemoveMaterialInput{Name: empty.Name, Version: empty.Version}
	if _, e = s.RemoveMaterial(ctx, removeKey, "remove", removeIn); e != nil {
		t.Fatal(e)
	}
	// Replaying the original add must not resurrect a deliberately removed name.
	if replayed, e := s.AddMaterial(ctx, k, "add", in); e != nil || !bytes.Equal(result, replayed) {
		t.Fatal(e)
	}
	items, e := s.Materials(ctx)
	if e != nil || slices.ContainsFunc(items, func(m MaterialUsage) bool { return m.Name == empty.Name }) {
		t.Fatal("old add recreated removed material", e)
	}
	if _, e = s.AddMaterial(ctx, ID(), ID(), in); e != nil {
		t.Fatal(e)
	}
	readded := usage(t, s, "新材质")
	if readded.Version == empty.Version {
		t.Fatal("re-add reused removed catalog identity")
	}
	if _, e = s.RemoveMaterial(ctx, removeKey, "remove", removeIn); e != nil || usage(t, s, "新材质") != readded {
		t.Fatal("old remove changed re-added material", e)
	}
	if _, e = s.RemoveMaterial(ctx, ID(), ID(), removeIn); e == nil {
		t.Fatal("stale empty catalog version accepted after re-add")
	}
	if _, e = s.AddMaterial(ctx, k, "different", AddMaterialInput{Name: "另一种"}); e == nil {
		t.Fatal("reused operation key accepted")
	}
	for _, name := range []string{"  ", strings.Repeat("材", 41)} {
		if _, e = s.AddMaterial(ctx, ID(), ID(), AddMaterialInput{Name: name}); e == nil {
			t.Fatal("invalid name accepted")
		}
	}
}

func TestMaterialCatalogMigrationAndBackup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f := save(t, s, example())
	if _, e := s.DB.Exec("DROP TABLE material_catalog"); e != nil {
		t.Fatal(e)
	}
	if e := s.Check(ctx); e == nil {
		t.Fatal("legacy database should request explicit migration")
	}
	oldBackup, oldRestore := filepath.Join(t.TempDir(), "old-backup"), filepath.Join(t.TempDir(), "old-restore")
	if e := s.Backup(ctx, oldBackup); e != nil {
		t.Fatal(e)
	}
	if e := Restore(ctx, oldBackup, oldRestore); e != nil {
		t.Fatal("cannot restore backup made before catalog migration", e)
	}
	if e := Migrate(ctx, oldRestore); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if e := Migrate(ctx, s.Dir); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.Check(ctx); e != nil {
		t.Fatal(e)
	}
	var version int
	if e := s.DB.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 1 {
		t.Fatal("migration broke v1 compatibility", e)
	}
	after, e := s.Get(ctx, f.ID)
	if e != nil || !reflect.DeepEqual(f, after) {
		t.Fatal("migration changed existing fabric", e)
	}
	if _, e = s.AddMaterial(ctx, ID(), ID(), AddMaterialInput{Name: "零使用材质"}); e != nil {
		t.Fatal(e)
	}
	empty := usage(t, s, "零使用材质")
	if e = s.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	backup, restored := filepath.Join(t.TempDir(), "backup"), filepath.Join(t.TempDir(), "restored")
	if e = s.Backup(ctx, backup); e != nil {
		t.Fatal(e)
	}
	if e = Restore(ctx, backup, restored); e != nil {
		t.Fatal(e)
	}
	r, e := Open(restored, false)
	if e != nil {
		t.Fatal(e)
	}
	defer r.DB.Close()
	if usage(t, r, "零使用材质") != empty {
		t.Fatal("backup lost standalone material identity")
	}
	if e = Migrate(ctx, filepath.Join(t.TempDir(), "missing")); e == nil {
		t.Fatal("migration created missing database")
	}
}
