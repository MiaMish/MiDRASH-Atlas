package atlas

import "testing"

func TestParseDatePreservesCircaAsQualifier(t *testing.T) {
	got := ParseDate("1460 בערך.")
	if got.Kind != "year" || got.Year == nil || *got.Year != 1460 || !got.Approximate {
		t.Fatalf("unexpected value: %+v", got)
	}
	if got.StartYear != nil || got.EndYear != nil {
		t.Fatalf("circa policy leaked into stored interval: %+v", got)
	}
}

func TestParseCentury(t *testing.T) {
	got := ParseDate(`מאה י"ד (מחצית שניה).`)
	if got.Kind != "century" || got.StartYear == nil || *got.StartYear != 1351 || got.EndYear == nil || *got.EndYear != 1400 {
		t.Fatalf("unexpected value: %+v", got)
	}
}

func TestParseParenthesizedRange(t *testing.T) {
	got := ParseDate(`א'תת"ט-א'תת"י לשטרות (1498-1499).`)
	if got.Kind != "range" || got.StartYear == nil || *got.StartYear != 1498 || got.EndYear == nil || *got.EndYear != 1499 {
		t.Fatalf("unexpected value: %+v", got)
	}
}

func TestParseQualifiedParenthesizedYear(t *testing.T) {
	got := ParseDate(`ק"ג (סוף 1342).`)
	if got.Kind != "year" || got.Year == nil || *got.Year != 1342 {
		t.Fatalf("unexpected value: %+v", got)
	}
}
