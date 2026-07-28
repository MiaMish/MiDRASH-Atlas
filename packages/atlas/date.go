package atlas

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	parenthesizedRange = regexp.MustCompile(`\((\d{3,4})-(\d{3,4})\)`)
	parenthesizedYear  = regexp.MustCompile(`\([^)]*?(\d{3,4})[^)]*?\)`)
	plainYearRange     = regexp.MustCompile(`^\s*(\d{3,4})-(\d{3,4})`)
	plainYear          = regexp.MustCompile(`^\s*(\d{3,4})`)
	centuryRange       = regexp.MustCompile(`מאה\s+([א-ת]["״']?[א-ת]?)\s*-\s*([א-ת]["״']?[א-ת]?)`)
	singleCentury      = regexp.MustCompile(`מאה\s+([א-ת]["״']?[א-ת]?)`)
)

func ParseDate(raw string) TemporalValue {
	v := TemporalValue{
		Kind:        "unparsed",
		Approximate: containsAny(raw, "בערך", "סביב"),
		Uncertain:   containsAny(raw, "(?)", "[?]", "(?)", "?"),
	}
	if m := parenthesizedRange.FindStringSubmatch(raw); len(m) == 3 {
		start, _ := strconv.Atoi(m[1])
		end, _ := strconv.Atoi(m[2])
		v.Kind, v.StartYear, v.EndYear = "range", intp(start), intp(end)
		return v
	}
	if m := parenthesizedYear.FindStringSubmatch(raw); len(m) == 2 {
		year, _ := strconv.Atoi(m[1])
		v.Kind, v.Year = "year", intp(year)
		return v
	}
	if m := plainYearRange.FindStringSubmatch(raw); len(m) == 3 {
		start, _ := strconv.Atoi(m[1])
		end, _ := strconv.Atoi(m[2])
		v.Kind, v.StartYear, v.EndYear = "range", intp(start), intp(end)
		return v
	}
	if m := plainYear.FindStringSubmatch(raw); len(m) == 2 {
		year, _ := strconv.Atoi(m[1])
		v.Kind, v.Year = "year", intp(year)
		return v
	}
	if m := centuryRange.FindStringSubmatch(normalizeHebrewNumerals(raw)); len(m) == 3 {
		start, ok1 := hebrewNumber(m[1])
		end, ok2 := hebrewNumber(m[2])
		if ok1 && ok2 {
			v.Kind = "century_range"
			v.StartCentury, v.EndCentury = intp(start), intp(end)
			v.StartYear, v.EndYear = intp((start-1)*100+1), intp(end*100)
			return v
		}
	}
	if m := singleCentury.FindStringSubmatch(normalizeHebrewNumerals(raw)); len(m) == 2 {
		c, ok := hebrewNumber(m[1])
		if ok {
			v.Kind, v.Century = "century", intp(c)
			switch {
			case strings.Contains(raw, "מחצית שניה"):
				v.Part = "second_half"
				v.StartYear, v.EndYear = intp((c-1)*100+51), intp(c*100)
			case strings.Contains(raw, "סוף"):
				v.Part = "end"
				v.StartYear, v.EndYear = intp(c*100-24), intp(c*100)
			default:
				v.StartYear, v.EndYear = intp((c-1)*100+1), intp(c*100)
			}
			return v
		}
	}
	return v
}

func intp(v int) *int { return &v }

func containsAny(s string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}

func normalizeHebrewNumerals(s string) string {
	r := strings.NewReplacer("״", "\"", "”", "\"", "׳", "'", "טו", "ט\"ו", "טז", "ט\"ז", "יח", "י\"ח", "יט", "י\"ט")
	return r.Replace(s)
}

func hebrewNumber(s string) (int, bool) {
	clean := strings.NewReplacer("\"", "", "'", "", "״", "", "׳", "").Replace(s)
	values := map[rune]int{'א': 1, 'ב': 2, 'ג': 3, 'ד': 4, 'ה': 5, 'ו': 6, 'ז': 7, 'ח': 8, 'ט': 9, 'י': 10, 'כ': 20}
	total := 0
	for _, r := range clean {
		n, ok := values[r]
		if !ok {
			return 0, false
		}
		total += n
	}
	return total, total > 0
}
