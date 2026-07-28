package atlas

type EffectiveInterval struct {
	StartYear int `json:"start_year"`
	EndYear   int `json:"end_year"`
}

// ApplyTemporalPolicy turns stored semantics into an application view.
// It never changes the persisted TemporalValue.
func ApplyTemporalPolicy(v TemporalValue, circaYears int) (EffectiveInterval, bool) {
	if circaYears < 0 {
		circaYears = 0
	}
	switch v.Kind {
	case "year":
		if v.Year == nil {
			return EffectiveInterval{}, false
		}
		start, end := *v.Year, *v.Year
		if v.Approximate {
			start -= circaYears
			end += circaYears
		}
		return EffectiveInterval{StartYear: start, EndYear: end}, true
	case "range", "century", "century_range":
		if v.StartYear == nil || v.EndYear == nil {
			return EffectiveInterval{}, false
		}
		start, end := *v.StartYear, *v.EndYear
		if v.Approximate {
			start -= circaYears
			end += circaYears
		}
		return EffectiveInterval{StartYear: start, EndYear: end}, true
	default:
		return EffectiveInterval{}, false
	}
}
