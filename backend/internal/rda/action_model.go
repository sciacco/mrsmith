package rda

import (
	"encoding/json"
	"strings"
)

const (
	poPermissionAvailable   = "available"
	poPermissionUnavailable = "unavailable"
)

func buildPOActionModel(po poDetail, permissions poActionPermissions, email string, quoteThreshold float64) poActionModel {
	if permissions.Status == "" {
		permissions.Status = poPermissionAvailable
	}

	model := poActionModel{
		PermissionStatus: permissions.Status,
		WorkflowStage:    poWorkflowStage(po.State),
		Summary: poActionSummary{
			State:           po.State,
			TotalPrice:      po.TotalPrice,
			Currency:        po.Currency,
			RowCount:        len(po.Rows),
			AttachmentCount: len(po.Attachments),
			QuoteCount:      countQuoteAttachments(po.Attachments),
			RecipientCount:  len(po.Recipients),
			PaymentMethod:   poPaymentMethodCode(po),
		},
	}

	addMode := func(mode poActionMode, actions ...poAction) {
		mode.ActionIDs = make([]string, 0, len(actions))
		for _, action := range actions {
			mode.ActionIDs = append(mode.ActionIDs, action.ID)
			model.Actions = append(model.Actions, action)
		}
		model.Modes = append(model.Modes, mode)
	}

	requester := isRequester(po, email)
	permissionReady := permissions.Status != poPermissionUnavailable
	roleUnavailableReason := ""
	if !permissionReady {
		roleUnavailableReason = "Le autorizzazioni non sono disponibili. Riprova piu tardi."
	}

	switch po.State {
	case "DRAFT":
		if requester {
			disabled, reason := submitDisabledReason(po, quoteThreshold)
			addMode(
				poActionMode{
					ID:          "requester_draft",
					Label:       "Richiedente",
					Description: "Completa la bozza e mandala in approvazione.",
					Reason:      "La richiesta e ancora modificabile.",
				},
				poAction{
					ID:             "submit",
					ModeID:         "requester_draft",
					Label:          "Manda in approvazione",
					Description:    "La richiesta passa agli approvatori assegnati.",
					Next:           "Approvazione",
					Tone:           "primary",
					Primary:        true,
					Disabled:       disabled,
					DisabledReason: reason,
				},
			)
		}
	case "PENDING_APPROVAL":
		if isApprover(po, email) {
			enabled := permissionReady && permissions.has(permissionApprover)
			disabledReason := actionDisabledReason(enabled, roleUnavailableReason, "Operazione riservata agli approvatori assegnati.")
			addMode(
				poActionMode{
					ID:          "approver_l1_l2",
					Label:       "Approvatore",
					Description: "Valuta la richiesta per il livello corrente.",
					Reason:      "Il PO attende approvazione.",
				},
				poAction{
					ID:             "approve",
					ModeID:         "approver_l1_l2",
					Label:          "Approva",
					Description:    "La richiesta passa allo step successivo.",
					Next:           "Prossimo controllo",
					Tone:           "primary",
					Primary:        true,
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
				poAction{
					ID:             "reject",
					ModeID:         "approver_l1_l2",
					Label:          "Rifiuta",
					Description:    "La richiesta viene fermata e resta visibile al richiedente.",
					Tone:           "danger",
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
			)
		}
	case "PENDING_APPROVAL_PAYMENT_METHOD":
		if requester {
			addMode(poActionMode{
				ID:          "requester_payment_update",
				Label:       "Richiedente",
				Description: "Correggi il metodo di pagamento richiesto.",
				Reason:      "Il metodo selezionato richiede una verifica.",
			})
		}
		if !permissionReady || permissions.has(permissionAFC) {
			enabled := permissionReady && permissions.has(permissionAFC)
			disabledReason := actionDisabledReason(enabled, roleUnavailableReason, "Operazione riservata agli utenti AFC.")
			addMode(
				poActionMode{
					ID:          "afc_payment",
					Label:       "AFC pagamento",
					Description: "Conferma o ferma il metodo di pagamento.",
					Reason:      "Il PO attende controllo sul pagamento.",
				},
				poAction{
					ID:             "payment-method/approve",
					ModeID:         "afc_payment",
					Label:          "Approva metodo",
					Description:    "Il metodo viene confermato e il PO avanza.",
					Next:           "Prossimo step",
					Tone:           "primary",
					Primary:        true,
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
				poAction{
					ID:             "reject",
					ModeID:         "afc_payment",
					Label:          "Rifiuta metodo",
					Description:    "La richiesta torna al richiedente per una correzione.",
					Tone:           "danger",
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
			)
		}
	case "PENDING_LEASING":
		if !permissionReady || permissions.has(permissionAFC) {
			enabled := permissionReady && permissions.has(permissionAFC)
			disabledReason := actionDisabledReason(enabled, roleUnavailableReason, "Operazione riservata agli utenti AFC.")
			addMode(
				poActionMode{
					ID:          "afc_leasing",
					Label:       "AFC leasing",
					Description: "Valuta il percorso leasing.",
					Reason:      "Il PO attende decisione sul leasing.",
				},
				poAction{
					ID:             "leasing/approve",
					ModeID:         "afc_leasing",
					Label:          "Approva leasing",
					Description:    "Il PO prosegue nel percorso leasing.",
					Next:           "Creazione ordine leasing",
					Tone:           "primary",
					Primary:        true,
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
				poAction{
					ID:             "leasing/reject",
					ModeID:         "afc_leasing",
					Label:          "Rifiuta leasing",
					Description:    "Il PO viene fermato per la decisione leasing.",
					Tone:           "danger",
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
			)
		}
	case "PENDING_LEASING_ORDER_CREATION":
		if !permissionReady || permissions.has(permissionAFC) {
			enabled := permissionReady && permissions.has(permissionAFC)
			addMode(
				poActionMode{
					ID:          "afc_leasing_created",
					Label:       "AFC leasing",
					Description: "Conferma la creazione del leasing.",
					Reason:      "Il PO attende conferma ordine leasing.",
				},
				poAction{
					ID:             "leasing/created",
					ModeID:         "afc_leasing_created",
					Label:          "Leasing creato",
					Description:    "Il PO avanza dopo la creazione del leasing.",
					Next:           "Invio",
					Tone:           "primary",
					Primary:        true,
					Disabled:       !enabled,
					DisabledReason: actionDisabledReason(enabled, roleUnavailableReason, "Operazione riservata agli utenti AFC."),
				},
			)
		}
	case "PENDING_APPROVAL_NO_LEASING":
		if !permissionReady || permissions.has(permissionApproverNoLeasing) {
			enabled := permissionReady && permissions.has(permissionApproverNoLeasing)
			disabledReason := actionDisabledReason(enabled, roleUnavailableReason, "Operazione riservata agli approvatori no leasing.")
			addMode(
				poActionMode{
					ID:          "no_leasing",
					Label:       "No leasing",
					Description: "Valuta il percorso senza leasing.",
					Reason:      "Il PO attende approvazione no leasing.",
				},
				poAction{
					ID:             "no-leasing/approve",
					ModeID:         "no_leasing",
					Label:          "Approva no leasing",
					Description:    "Il PO prosegue senza leasing.",
					Next:           "Invio",
					Tone:           "primary",
					Primary:        true,
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
				poAction{
					ID:             "reject",
					ModeID:         "no_leasing",
					Label:          "Rifiuta",
					Description:    "La richiesta viene fermata.",
					Tone:           "danger",
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
			)
		}
	case "PENDING_BUDGET_INCREMENT":
		if !permissionReady || permissions.has(permissionExtraBudget) {
			enabled := permissionReady && permissions.has(permissionExtraBudget)
			disabledReason := actionDisabledReason(enabled, roleUnavailableReason, "Operazione riservata agli approvatori budget.")
			addMode(
				poActionMode{
					ID:          "extra_budget",
					Label:       "Extra budget",
					Description: "Valuta la promessa di incremento budget.",
					Reason:      "Il PO attende copertura budget.",
				},
				poAction{
					ID:             "budget-increment/approve",
					ModeID:         "extra_budget",
					Label:          "Approva incremento",
					Description:    "La copertura budget viene accettata.",
					Next:           "Invio",
					Tone:           "primary",
					Primary:        true,
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
				poAction{
					ID:             "budget-increment/reject",
					ModeID:         "extra_budget",
					Label:          "Rifiuta incremento",
					Description:    "La richiesta viene fermata per mancanza di copertura.",
					Tone:           "danger",
					Disabled:       !enabled,
					DisabledReason: disabledReason,
				},
			)
		}
	case "PENDING_SEND":
		addMode(
			poActionMode{
				ID:          "send_provider",
				Label:       "Invio fornitore",
				Description: "Invia l'ordine al fornitore.",
				Reason:      "Il PO e pronto per l'invio.",
			},
			poAction{
				ID:          "send-to-provider",
				ModeID:      "send_provider",
				Label:       "Invia al fornitore",
				Description: "Il fornitore riceve l'ordine.",
				Next:        "Verifica conformita",
				Tone:        "primary",
				Primary:     true,
			},
		)
	case "PENDING_VERIFICATION":
		addMode(
			poActionMode{
				ID:          "conformity",
				Label:       "Verifica fornitura",
				Description: "Conferma la conformita o apri una contestazione.",
				Reason:      "Il PO attende verifica finale.",
			},
			poAction{
				ID:          "conformity/confirm",
				ModeID:      "conformity",
				Label:       "Erogato e conforme",
				Description: "La richiesta puo essere chiusa.",
				Next:        "Chiusura",
				Tone:        "primary",
				Primary:     true,
			},
			poAction{
				ID:          "conformity/reject",
				ModeID:      "conformity",
				Label:       "In contestazione",
				Description: "La fornitura viene segnalata come non conforme.",
				Tone:        "danger",
			},
		)
	}

	model.PrimaryModeID = selectPrimaryModeID(model.Modes)
	if len(model.Modes) == 0 {
		model.Modes = []poActionMode{{
			ID:          "read_only",
			Label:       "Consultazione",
			Description: "Segui lo stato della richiesta.",
			Reason:      "Non ci sono azioni disponibili per il tuo profilo in questo momento.",
		}}
		model.PrimaryModeID = "read_only"
	}
	return model
}

func addPOActionModel(body []byte, model poActionModel) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	encodedModel, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	payload["action_model"] = encodedModel
	return json.Marshal(payload)
}

func submitDisabledReason(po poDetail, quoteThreshold float64) (bool, string) {
	if len(po.Rows) == 0 {
		return true, "Aggiungi almeno una riga PO."
	}
	if parseTotalPrice(po.TotalPrice) >= quoteThreshold && countQuoteAttachments(po.Attachments) < 2 {
		return true, "Carica almeno 2 preventivi per inviare la richiesta."
	}
	return false, ""
}

func actionDisabledReason(enabled bool, unavailableReason string, forbiddenReason string) string {
	if enabled {
		return ""
	}
	if unavailableReason != "" {
		return unavailableReason
	}
	return forbiddenReason
}

func selectPrimaryModeID(modes []poActionMode) string {
	if len(modes) == 0 {
		return ""
	}
	for _, mode := range modes {
		if mode.ID == "requester_draft" || mode.ID == "requester_payment_update" {
			return mode.ID
		}
	}
	return modes[0].ID
}

func poWorkflowStage(state string) string {
	switch strings.TrimSpace(state) {
	case "DRAFT":
		return "draft"
	case "PENDING_APPROVAL", "APPROVED":
		return "approval"
	case "PENDING_APPROVAL_PAYMENT_METHOD", "PENDING_LEASING", "PENDING_LEASING_ORDER_CREATION", "PENDING_APPROVAL_NO_LEASING", "PENDING_BUDGET_INCREMENT":
		return "method_budget"
	case "PENDING_SEND":
		return "send"
	case "PENDING_VERIFICATION", "SENT":
		return "verification"
	case "CLOSED", "REJECTED":
		return "closure"
	default:
		return "draft"
	}
}
