package training

// Stati di erogazione dell'iscrizione (training.enrollment.delivery_status).
const (
	deliveryPlanned            = "planned"
	deliveryInProgress         = "in_progress"
	deliveryCompleted          = "completed"
	deliveryPartiallyCompleted = "partially_completed"
	deliveryNotAttended        = "not_attended"
	deliveryCancelled          = "cancelled"
)

// Stati di partecipazione (training.enrollment_session.participation_status).
const (
	participationAssigned    = "assigned"
	participationInProgress  = "in_progress"
	participationCompleted   = "completed"
	participationNotAttended = "not_attended"
)

// computeDeliveryStatus deriva lo stato di erogazione di un'iscrizione dalle
// sue partecipazioni. E l'unico punto di calcolo degli stati «di fatto»:
// le azioni di dominio e i futuri import lo riusano senza duplicarlo.
// Il tempo non entra mai nel calcolo; `cancelled` e lo storico-completato
// sono stati decisi, scritti dalle rispettive azioni e mai da questa funzione.
func computeDeliveryStatus(participationStatuses []string) string {
	if len(participationStatuses) == 0 {
		return deliveryPlanned
	}
	allAssigned := true
	allTerminal := true
	anyCompleted := false
	anyNotAttended := false
	for _, status := range participationStatuses {
		switch status {
		case participationAssigned:
			allTerminal = false
		case participationInProgress:
			allAssigned = false
			allTerminal = false
		case participationCompleted:
			allAssigned = false
			anyCompleted = true
		case participationNotAttended:
			allAssigned = false
			anyNotAttended = true
		}
	}
	if allAssigned {
		return deliveryPlanned
	}
	if !allTerminal {
		return deliveryInProgress
	}
	if anyCompleted && anyNotAttended {
		return deliveryPartiallyCompleted
	}
	if anyNotAttended {
		return deliveryNotAttended
	}
	return deliveryCompleted
}
