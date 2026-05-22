package training

import "testing"

func TestNormalizePopulationTarget(t *testing.T) {
	cases := []struct {
		name    string
		input   PopulationTarget
		want    PopulationTarget
		wantErr bool
	}{
		{
			name:  "default all",
			input: PopulationTarget{},
			want:  PopulationTarget{Kind: "all"},
		},
		{
			name:  "team target",
			input: PopulationTarget{Kind: "team", ID: "team-1"},
			want:  PopulationTarget{Kind: "team", ID: "team-1"},
		},
		{
			name:    "team requires id",
			input:   PopulationTarget{Kind: "team"},
			want:    PopulationTarget{Kind: "team"},
			wantErr: true,
		},
		{
			name:    "role is not a population kind",
			input:   PopulationTarget{Kind: "role", ID: "developer"},
			want:    PopulationTarget{Kind: "role", ID: "developer"},
			wantErr: true,
		},
		{
			name:    "hire date is not a population kind",
			input:   PopulationTarget{Kind: "hire_date", ID: "2026-01-01"},
			want:    PopulationTarget{Kind: "hire_date", ID: "2026-01-01"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizePopulationTarget(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind != tc.want.Kind || got.ID != tc.want.ID {
				t.Fatalf("target = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestMandatoryRuleWarnings(t *testing.T) {
	if got := mandatoryRuleWarnings(MandatoryRule{Active: false, TargetCount: 0, GapCount: 3}); len(got) != 0 {
		t.Fatalf("inactive warnings = %#v, want none", got)
	}

	got := mandatoryRuleWarnings(MandatoryRule{Active: true, TargetCount: 0, GapCount: 0})
	if len(got) != 1 || got[0] != "empty_population" {
		t.Fatalf("empty population warnings = %#v", got)
	}

	got = mandatoryRuleWarnings(MandatoryRule{Active: true, TargetCount: 10, GapCount: 2})
	if len(got) != 1 || got[0] != "coverage_gap" {
		t.Fatalf("coverage warnings = %#v", got)
	}
}
