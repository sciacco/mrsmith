package rda

import "strings"

var poPDFDownloadStates = map[string]struct{}{
	"APPROVED":                {},
	"PENDING_SEND":            {},
	"SENT":                    {},
	"PENDING_VERIFICATION":    {},
	"PENDING_DISPUTE":         {},
	"DELIVERED_AND_COMPLIANT": {},
	"CLOSED":                  {},
}

func canDownloadPOPDF(state string) bool {
	_, ok := poPDFDownloadStates[strings.TrimSpace(state)]
	return ok
}
