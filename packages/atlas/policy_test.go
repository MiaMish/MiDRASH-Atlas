package atlas

import "testing"

func TestApplyTemporalPolicy(t *testing.T) {
	year := 1460
	stored := TemporalValue{Kind: "year", Year: &year, Approximate: true}
	got, ok := ApplyTemporalPolicy(stored, 10)
	if !ok || got.StartYear != 1450 || got.EndYear != 1470 {
		t.Fatalf("unexpected interval: %+v, %v", got, ok)
	}
	if stored.StartYear != nil || stored.EndYear != nil || *stored.Year != 1460 {
		t.Fatalf("policy mutated stored semantics: %+v", stored)
	}
}
