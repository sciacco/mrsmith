package training

import "testing"

func TestComputeDeliveryStatus(t *testing.T) {
	cases := []struct {
		name           string
		participations []string
		want           string
	}{
		{"nessuna partecipazione", nil, deliveryPlanned},
		{"una assegnata", []string{"assigned"}, deliveryPlanned},
		{"tutte assegnate", []string{"assigned", "assigned", "assigned"}, deliveryPlanned},
		{"una in corso", []string{"in_progress"}, deliveryInProgress},
		{"assegnata e in corso", []string{"assigned", "in_progress"}, deliveryInProgress},
		{"conclusa e assegnata", []string{"completed", "assigned"}, deliveryInProgress},
		{"non frequentata e assegnata", []string{"not_attended", "assigned"}, deliveryInProgress},
		{"conclusa e in corso", []string{"completed", "in_progress"}, deliveryInProgress},
		{"terminali coerenti: tutte completate", []string{"completed"}, deliveryCompleted},
		{"terminali coerenti: piu completate", []string{"completed", "completed"}, deliveryCompleted},
		{"terminali coerenti: tutte non frequentate", []string{"not_attended"}, deliveryNotAttended},
		{"terminali coerenti: piu non frequentate", []string{"not_attended", "not_attended"}, deliveryNotAttended},
		{"terminali miste", []string{"completed", "not_attended"}, deliveryPartiallyCompleted},
		{"terminali miste con piu righe", []string{"completed", "not_attended", "completed"}, deliveryPartiallyCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeDeliveryStatus(tc.participations); got != tc.want {
				t.Fatalf("computeDeliveryStatus(%v) = %q, want %q", tc.participations, got, tc.want)
			}
		})
	}
}
