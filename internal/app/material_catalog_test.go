package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"reflect"
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
}
