package app

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestMaterialPercentages(t *testing.T) {
	f := example()
	f.MaterialPercentages = map[string]string{"棉": "75.1250", "麻": "50", "丝": "12"}
	if e := f.Validate(); e != nil {
		t.Fatal("a total above 100 must be accepted", e)
	}
	want := map[string]string{"棉": "75.1250", "麻": "50"}
	if !reflect.DeepEqual(f.MaterialPercentages, want) || f.MaterialText() != "棉 75.1250%、麻 50%" {
		t.Fatal("percentages lost precision or unselected material retained", f.MaterialPercentages)
	}
	for _, value := range []string{"", "0", "150.5"} {
		f.MaterialPercentages["棉"] = value
		if e := f.Validate(); e != nil || f.MaterialPercentages["棉"] != value {
			t.Fatal("percentage was changed or rejected", value, e)
		}
	}
	for _, value := range []string{"NaN", "1e2", "1.2.3", "-5"} {
		f.MaterialPercentages["棉"] = value
		if e := f.Validate(); e == nil {
			t.Fatal("accepted a non-decimal percentage", value)
		}
	}
	f.Materials = []string{"棉"}
	if e := f.Validate(); e != nil || !reflect.DeepEqual(f.MaterialPercentages, map[string]string{"棉": "100"}) {
		t.Fatal("single material must be 100%", e)
	}
}

func TestMaterialPercentagesSurviveEditsAndRemnants(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f := example()
	f.Composition = "表层棉麻，背面有涂层"
	f.MaterialPercentages = map[string]string{"棉": "70.25", "麻": "10"}
	f = save(t, s, f)
	want := f.MaterialPercentages
	// A cached older page sends no ratios while changing other details.
	f.MaterialPercentages = nil
	f.Notes = "旧页面修改备注"
	f = save(t, s, f)
	if !reflect.DeepEqual(f.MaterialPercentages, want) {
		t.Fatal("an old client cleared the ratios")
	}
	b, e := s.Write(ctx, f.ID, ID(), ID(), "remnant", Fabric{Revision: f.Revision, Status: "using", Pieces: f.Pieces, MaterialPercentages: map[string]string{"棉": "1"}})
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(b, &f)
	loaded, e := s.Get(ctx, f.ID)
	if e != nil || !reflect.DeepEqual(loaded.MaterialPercentages, want) || loaded.Composition != "表层棉麻，背面有涂层" {
		t.Fatal("remnant edit lost material details", e)
	}
	loaded.MaterialPercentages = map[string]string{}
	loaded = save(t, s, loaded)
	if loaded.MaterialPercentages["棉"] != "" || loaded.MaterialPercentages["麻"] != "" {
		t.Fatal("explicitly clearing ratios failed")
	}
	loaded.Materials = []string{"麻"}
	loaded = save(t, s, loaded)
	if !reflect.DeepEqual(loaded.MaterialPercentages, map[string]string{"麻": "100"}) {
		t.Fatal("removed material retained or single material not defaulted")
	}
}
