package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestLedgerImport(t *testing.T) {
	s := testStore(t)
	h := s.Handler(nil)
	ctx := context.Background()
	call := func(id, name string) Fabric {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"transactionId": id, "name": name, "purchaseDate": "2026-09-19", "price": "123.45"})
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("POST", "/api/integrations/ledger", strings.NewReader(string(body))))
		if r.Code != 200 {
			t.Errorf("%d: %s", r.Code, r.Body.String())
			return Fabric{}
		}
		var f Fabric
		if e := json.Unmarshal(r.Body.Bytes(), &f); e != nil {
			t.Error(e)
		}
		return f
	}
	id := "00000000-0000-4000-8000-000000000001"
	f := call(id, strings.Repeat("布", 500))
	if len([]rune(f.Name)) != 500 || f.PurchaseDate != "2026-09-19" || *f.PriceCents != 12345 || f.Status != "unused" || len(f.Pieces) != 1 || f.Pieces[0].WidthMM != nil || f.Pieces[0].LengthMM != nil || len(f.PhotoIDs) != 0 || f.Location != "" || f.Notes != "" || len(f.Materials) != 0 {
		t.Fatal(f)
	}
	f.Name, f.Location = "补填名称", "收纳箱"
	f = save(t, s, f)
	if _, e := s.DB.Exec("UPDATE operations SET created_at='2000-01-01T00:00:00Z'"); e != nil {
		t.Fatal(e)
	}
	if e := s.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := call(id, "重试的新标题"); got.ID != f.ID {
				t.Error("duplicate fabric")
			}
		}()
	}
	wg.Wait()
	got, e := s.Get(ctx, f.ID)
	if e != nil || got.Name != f.Name || got.Location != f.Location || got.Revision != 2 {
		t.Fatal(got, e)
	}
	var count int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM fabrics").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	if _, e = s.Write(ctx, f.ID, ID(), ID(), "delete", f); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("POST", "/api/integrations/ledger", strings.NewReader(fmt.Sprintf(`{"transactionId":%q,"name":"deleted","purchaseDate":"2026-09-19","price":"1"}`, id))))
	if r.Code != 409 {
		t.Fatal("deleted fabric should not be recreated", r.Code)
	}
	if got := call("00000000-0000-4000-8000-000000000002", ""); got.Name != "未命名布料" {
		t.Fatal(got)
	}
	for _, body := range []string{`{}`, `{"transactionId":"invalid"}`, fmt.Sprintf(`{"transactionId":%q,"name":"x","purchaseDate":"bad","price":"1"}`, "00000000-0000-4000-8000-000000000003")} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("POST", "/api/integrations/ledger", strings.NewReader(body)))
		if r.Code < 400 {
			t.Fatal("invalid import accepted", body)
		}
	}
}
