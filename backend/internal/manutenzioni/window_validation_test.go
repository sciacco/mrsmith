package manutenzioni

import (
	"errors"
	"testing"
)

func TestValidateWindowRequestRequiresEndAfterStart(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		end     string
		wantErr bool
	}{
		{
			name:  "valid range",
			start: "2026-05-26T13:23",
			end:   "2026-05-26T15:00",
		},
		{
			name:    "equal timestamps",
			start:   "2026-05-26T13:23",
			end:     "2026-05-26T13:23",
			wantErr: true,
		},
		{
			name:    "end before start",
			start:   "2026-05-26T15:00",
			end:     "2026-05-26T13:23",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, _, _, err := validateWindowRequest(windowRequest{
				ScheduledStartAt: tc.start,
				ScheduledEndAt:   tc.end,
			})
			if tc.wantErr {
				if !errors.Is(err, errInvalidWindowRange) {
					t.Fatalf("err = %v, want errInvalidWindowRange", err)
				}
				if got := windowRequestErrorCode(err); got != "invalid_window_range" {
					t.Fatalf("error code = %q, want invalid_window_range", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateWindowRequest returned error: %v", err)
			}
		})
	}
}

func TestValidateWindowRequestKeepsGenericInvalidWindowForParseErrors(t *testing.T) {
	_, _, _, _, _, _, err := validateWindowRequest(windowRequest{
		ScheduledStartAt: "not-a-date",
		ScheduledEndAt:   "2026-05-26T15:00",
	})
	if !errors.Is(err, errBadRequest) {
		t.Fatalf("err = %v, want errBadRequest", err)
	}
	if got := windowRequestErrorCode(err); got != "invalid_window" {
		t.Fatalf("error code = %q, want invalid_window", got)
	}
}
