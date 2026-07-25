package binocolo

import (
	"errors"
	"testing"
	"time"
)

// Verifica minima del registro identità azienda (issue #86), parte pura.
//
// Quello che NON sta qui perché richiede il DB — trigger di immutabilità e
// divieto di regressione dello stato, idempotenza del ledger, race di due
// resolver sulla stessa nuova identità, round-trip dell'UUID attraverso target/
// rating/web validation, gate di equivalenza — vive nel runbook di cutover e
// nella sonda apps/binocolo/docs/IDENTITA-AZIENDA-PROBE.sql, eseguita
// dall'utente sui dati reali.

func TestExtractMACompanyIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		vat      string
		tax      string
		vendor   string
		expected []maCompanyIdentifier
	}{
		{
			name: "prefisso IT rimosso e punteggiatura pulita",
			vat:  " it 012 345 678.90 ",
			expected: []maCompanyIdentifier{
				{maCompanyIdentifierRef{maIdentifierNamespaceFiscal, "01234567890"}, true, false},
			},
		},
		{
			name: "P.IVA con prefisso IT canonico",
			vat:  "IT01234567890",
			expected: []maCompanyIdentifier{
				{maCompanyIdentifierRef{maIdentifierNamespaceFiscal, "01234567890"}, true, false},
			},
		},
		{
			name: "P.IVA e CF coincidenti: UNA riga con entrambi i ruoli",
			vat:  "01234567890",
			tax:  "0123456789 0",
			expected: []maCompanyIdentifier{
				{maCompanyIdentifierRef{maIdentifierNamespaceFiscal, "01234567890"}, true, true},
			},
		},
		{
			name: "P.IVA e CF distinti: DUE righe nello stesso namespace",
			vat:  "01234567890",
			tax:  "rssmra80a01h501u",
			expected: []maCompanyIdentifier{
				{maCompanyIdentifierRef{maIdentifierNamespaceFiscal, "01234567890"}, true, false},
				{maCompanyIdentifierRef{maIdentifierNamespaceFiscal, "RSSMRA80A01H501U"}, false, true},
			},
		},
		{
			name:   "vendor id nel proprio namespace, nella forma con cui è stato adottato",
			vendor: " 60c8f1a2b3c4d5e6f7a8b9c0 ",
			tax:    "RSSMRA80A01H501U",
			expected: []maCompanyIdentifier{
				{maCompanyIdentifierRef{maIdentifierNamespaceFiscal, "RSSMRA80A01H501U"}, false, true},
				{maCompanyIdentifierRef{maIdentifierNamespaceVendor, "60C8F1A2B3C4D5E6F7A8B9C0"}, false, false},
			},
		},
		{
			name:     "nessun identificatore: la ragione sociale non è identità",
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := extractMACompanyIdentifiers(test.vat, test.tax, test.vendor)
			if len(got) != len(test.expected) {
				t.Fatalf("identificatori = %+v, atteso %+v", got, test.expected)
			}
			for index := range got {
				if got[index] != test.expected[index] {
					t.Fatalf("identificatore %d = %+v, atteso %+v", index, got[index], test.expected[index])
				}
			}
			// Idempotenza: rinormalizzare i valori già puliti non li cambia.
			for _, identifier := range got {
				if identifier.Namespace != maIdentifierNamespaceFiscal {
					continue
				}
				if again := maStableVAT(identifier.Value); again != identifier.Value {
					t.Fatalf("normalizzazione non idempotente: %q -> %q", identifier.Value, again)
				}
			}
		})
	}
}

func fixedKeyMinter(prefix string) func() string {
	count := 0
	return func() string {
		count++
		return prefix + string(rune('0'+count))
	}
}

func TestPlanMACompanyResolutionAssignsNewIdentity(t *testing.T) {
	plan := planMACompanyResolution(
		[]maCompanyObservation{{VATCode: "01234567890", VendorID: "ABC", CompanyName: "Acme"}},
		map[maCompanyIdentifierRef]string{},
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 0 || len(plan.MissingIndex) != 0 {
		t.Fatalf("piano inatteso: %+v", plan)
	}
	if plan.Keys[0] != "new-1" {
		t.Fatalf("chiave = %q, attesa new-1", plan.Keys[0])
	}
	if len(plan.NewEntities) != 1 || plan.NewEntities[0].IdentityState != maCompanyIdentityStateFiscal {
		t.Fatalf("entità = %+v, attesa una sola con stato fiscal", plan.NewEntities)
	}
	if len(plan.Attachments) != 2 {
		t.Fatalf("attachment = %+v, attesi 2 (fiscale + vendor)", plan.Attachments)
	}
}

func TestNewMACompanyKeyIsAlreadyNormalized(t *testing.T) {
	// Le chiavi in arrivo passano ovunque da normalizeMACompanyKey (upper+trim):
	// un UUID minuscolo verrebbe maiuscolizzato al primo confronto e non
	// ritroverebbe più la propria riga.
	key := newMACompanyKey()
	if normalizeMACompanyKey(key) != key {
		t.Fatalf("chiave coniata non canonica: %q -> %q", key, normalizeMACompanyKey(key))
	}
	if key == newMACompanyKey() {
		t.Fatalf("due chiamate hanno coniato la stessa chiave %q", key)
	}
}

func TestPlanMACompanyResolutionVendorOnlyIsAnExplicitState(t *testing.T) {
	plan := planMACompanyResolution(
		[]maCompanyObservation{{VendorID: "60C8F1A2B3C4D5E6F7A8B9C0", CompanyName: "Senza fisco"}},
		map[maCompanyIdentifierRef]string{},
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.NewEntities) != 1 {
		t.Fatalf("entità = %+v", plan.NewEntities)
	}
	if plan.NewEntities[0].IdentityState != maCompanyIdentityStateVendorOnly {
		t.Fatalf("stato = %q, atteso vendor_only", plan.NewEntities[0].IdentityState)
	}
}

func TestPlanMACompanyResolutionMissingIdentityErrsInsteadOfDeriving(t *testing.T) {
	plan := planMACompanyResolution(
		[]maCompanyObservation{{CompanyName: "Solo un nome"}},
		map[maCompanyIdentifierRef]string{},
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.MissingIndex) != 1 || plan.MissingIndex[0] != 0 {
		t.Fatalf("atteso il rifiuto dell'osservazione senza identificatori, ottenuto %+v", plan)
	}
	if len(plan.NewEntities) != 0 {
		t.Fatalf("nessuna entità doveva nascere da una ragione sociale: %+v", plan.NewEntities)
	}
}

func TestPlanMACompanyResolutionAttachesNewVendorIDToExistingCompany(t *testing.T) {
	// Il giorno del cambio fornitore: la P.IVA è nota, il vendor id è nuovo.
	// L'azienda deve restare la stessa e il nuovo id diventare storico.
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceFiscal, "01234567890"}: "ADOPTED-KEY",
	}
	plan := planMACompanyResolution(
		[]maCompanyObservation{{VATCode: "01234567890", VendorID: "NUOVO-ID", CompanyName: "Acme"}},
		existing,
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 0 {
		t.Fatalf("conflitti inattesi: %+v", plan.Conflicts)
	}
	if plan.Keys[0] != "ADOPTED-KEY" {
		t.Fatalf("chiave = %q, attesa ADOPTED-KEY", plan.Keys[0])
	}
	if len(plan.NewEntities) != 0 {
		t.Fatalf("nessuna entità nuova doveva nascere: %+v", plan.NewEntities)
	}
	found := false
	for _, attachment := range plan.Attachments {
		if attachment.Namespace == maIdentifierNamespaceVendor && attachment.Value == "NUOVO-ID" {
			found = true
			if attachment.CompanyKey != "ADOPTED-KEY" {
				t.Fatalf("il vendor id nuovo è finito su %q", attachment.CompanyKey)
			}
		}
	}
	if !found {
		t.Fatalf("attach-on-found mancante: %+v", plan.Attachments)
	}
}

func TestPlanMACompanyResolutionPromotesVendorOnlyOnFiscalAttach(t *testing.T) {
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceVendor, "VENDOR-1"}: "KEY-1",
	}
	plan := planMACompanyResolution(
		[]maCompanyObservation{{VendorID: "VENDOR-1", VATCode: "01234567890"}},
		existing,
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Promotions) != 1 || plan.Promotions[0] != "KEY-1" {
		t.Fatalf("promozione attesa per KEY-1, ottenuto %+v", plan.Promotions)
	}
}

func TestPlanMACompanyResolutionDedupeIsNotAConflict(t *testing.T) {
	// Il caso gemello che NON deve errare: due occorrenze dello stesso batch che
	// mappano sulla STESSA azienda sono deduplica, non collisione.
	plan := planMACompanyResolution(
		[]maCompanyObservation{
			{VATCode: "01234567890", VendorID: "V1", CompanyName: "Acme"},
			{VATCode: "01234567890", VendorID: "V1", CompanyName: "Acme S.p.A."},
		},
		map[maCompanyIdentifierRef]string{},
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 0 {
		t.Fatalf("la deduplica non è un conflitto: %+v", plan.Conflicts)
	}
	if plan.Keys[0] != plan.Keys[1] {
		t.Fatalf("chiavi divergenti per la stessa azienda: %q vs %q", plan.Keys[0], plan.Keys[1])
	}
	if len(plan.NewEntities) != 1 {
		t.Fatalf("una sola entità attesa, ottenute %d", len(plan.NewEntities))
	}
}

func TestPlanMACompanyResolutionConflictAgainstRegistry(t *testing.T) {
	// Una sola occorrenza i cui identificatori appartengono a due aziende già
	// registrate: il vendor id a KEY-A, la P.IVA a KEY-B.
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceVendor, "V1"}:          "KEY-A",
		{maIdentifierNamespaceFiscal, "01234567890"}: "KEY-B",
	}
	plan := planMACompanyResolution(
		[]maCompanyObservation{{VendorID: "V1", VATCode: "01234567890", CompanyName: "Ambigua", TargetID: "t-1"}},
		existing,
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 1 {
		t.Fatalf("un conflitto atteso, ottenuti %+v", plan.Conflicts)
	}
	if plan.Keys[0] != "" {
		t.Fatalf("un'osservazione in conflitto non deve avere chiave: %q", plan.Keys[0])
	}
	if plan.Conflicts[0].TargetID != "t-1" {
		t.Fatalf("diagnostica dell'occorrenza persa: %+v", plan.Conflicts[0])
	}
}

func TestPlanMACompanyResolutionIntraBatchConflict(t *testing.T) {
	// Conflitto che nasce DENTRO il batch, contro un registro vuoto: le prime
	// due osservazioni coniano due aziende distinte, la terza rivendica un
	// identificatore di ciascuna.
	plan := planMACompanyResolution(
		[]maCompanyObservation{
			{VATCode: "01234567890", CompanyName: "Alfa"},
			{VATCode: "09876543210", CompanyName: "Beta"},
			{VATCode: "01234567890", TaxCode: "09876543210", CompanyName: "Ambigua", TargetID: "t-3"},
		},
		map[maCompanyIdentifierRef]string{},
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 1 {
		t.Fatalf("un conflitto intra-batch atteso, ottenuti %+v", plan.Conflicts)
	}
	if plan.Conflicts[0].TargetID != "t-3" {
		t.Fatalf("il conflitto va attribuito alla terza osservazione: %+v", plan.Conflicts[0])
	}
	if plan.Keys[0] == "" || plan.Keys[1] == "" || plan.Keys[0] == plan.Keys[1] {
		t.Fatalf("le prime due osservazioni sono aziende distinte e valide: %+v", plan.Keys)
	}
	if plan.Keys[2] != "" {
		t.Fatalf("l'osservazione in conflitto non deve avere chiave: %q", plan.Keys[2])
	}
}

func TestPlanMACompanyResolutionPriorKeyIsNeverSilentlyReplaced(t *testing.T) {
	// Il caso che re-chiaverebbe una riga persistita: il target porta KEY-A, ma
	// la P.IVA osservata risulta registrata su KEY-B. Sostituire in silenzio
	// lascerebbe rating e dossier su KEY-A — l'orfanamento che la issue
	// impedisce. Deve essere un conflitto.
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceFiscal, "01234567890"}: "KEY-B",
	}
	plan := planMACompanyResolution(
		[]maCompanyObservation{{PriorCompanyKey: "KEY-A", VATCode: "01234567890", TargetID: "t-1"}},
		existing,
		map[string]bool{"KEY-A": true, "KEY-B": true},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 1 {
		t.Fatalf("un conflitto atteso, ottenuti %+v", plan.Conflicts)
	}
	if plan.Conflicts[0].ExistingCompanyKey != "KEY-B" || plan.Conflicts[0].IncomingCompanyKey != "KEY-A" {
		t.Fatalf("diagnostica invertita: %+v", plan.Conflicts[0])
	}
	if plan.Keys[0] != "" {
		t.Fatalf("nessuna chiave per un'osservazione in conflitto: %q", plan.Keys[0])
	}
	if len(plan.Attachments) != 0 {
		t.Fatalf("nessun attach prima di una decisione umana: %+v", plan.Attachments)
	}
}

func TestPlanMACompanyResolutionPriorKeySeedsResolutionWithoutConflict(t *testing.T) {
	// Il caso normale: la chiave portata e gli identificatori concordano, e un
	// identificatore nuovo si aggancia alla stessa azienda.
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceFiscal, "01234567890"}: "KEY-A",
	}
	plan := planMACompanyResolution(
		[]maCompanyObservation{{PriorCompanyKey: "KEY-A", VATCode: "01234567890", VendorID: "V-NUOVO"}},
		existing,
		map[string]bool{"KEY-A": true},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 0 {
		t.Fatalf("nessun conflitto atteso: %+v", plan.Conflicts)
	}
	if plan.Keys[0] != "KEY-A" {
		t.Fatalf("chiave = %q, attesa KEY-A", plan.Keys[0])
	}
	found := false
	for _, attachment := range plan.Attachments {
		if attachment.Namespace == maIdentifierNamespaceVendor && attachment.CompanyKey == "KEY-A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("il vendor id nuovo doveva agganciarsi a KEY-A: %+v", plan.Attachments)
	}
}

func TestPlanMACompanyResolutionReportsEveryConflict(t *testing.T) {
	// È il test che smaschera un'implementazione che si affida alla prima unique
	// violation: PostgreSQL abortirebbe la transazione e il secondo conflitto
	// non sarebbe più raccoglibile.
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceVendor, "V1"}:          "KEY-A",
		{maIdentifierNamespaceFiscal, "01234567890"}: "KEY-B",
		{maIdentifierNamespaceVendor, "V2"}:          "KEY-C",
		{maIdentifierNamespaceFiscal, "09876543210"}: "KEY-D",
	}
	plan := planMACompanyResolution(
		[]maCompanyObservation{
			{VendorID: "V1", VATCode: "01234567890", TargetID: "t-1"},
			{VendorID: "V2", VATCode: "09876543210", TargetID: "t-2"},
		},
		existing,
		map[string]bool{},
		fixedKeyMinter("new-"),
	)
	if len(plan.Conflicts) != 2 {
		t.Fatalf("attesi 2 conflitti, ottenuti %+v", plan.Conflicts)
	}
	err := &maCompanyIdentityConflictError{Conflicts: plan.Conflicts}
	var typed *maCompanyIdentityConflictError
	if !errors.As(error(err), &typed) || len(typed.Conflicts) != 2 {
		t.Fatalf("errore tipizzato incompleto: %v", err)
	}
}

func TestPlanMACompanyResolutionReusesPriorKeyOnRepersist(t *testing.T) {
	// Re-persistenza di una riga riletta dal DB: senza identificatori
	// utilizzabili la chiave pregressa vale, e non si conia un'identità nuova a
	// ogni salvataggio.
	plan := planMACompanyResolution(
		[]maCompanyObservation{{PriorCompanyKey: "OLD-KEY", CompanyName: "Storica"}},
		map[maCompanyIdentifierRef]string{},
		map[string]bool{"OLD-KEY": true},
		fixedKeyMinter("new-"),
	)
	if plan.Keys[0] != "OLD-KEY" {
		t.Fatalf("chiave = %q, attesa OLD-KEY", plan.Keys[0])
	}
	if len(plan.NewEntities) != 0 {
		t.Fatalf("nessuna entità nuova attesa: %+v", plan.NewEntities)
	}
}

func TestPlanMACompanyResolutionAttachmentsCarryTheObservationInstant(t *testing.T) {
	// first_seen_at/last_seen_at datano l'osservazione del fornitore, non il
	// passaggio nel resolver: una ri-persistenza non deve poterli avanzare.
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceFiscal, "01234567890"}: "KEY-A",
	}
	observedAt := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)

	repersist := planMACompanyResolution(
		[]maCompanyObservation{{PriorCompanyKey: "KEY-A", VATCode: "01234567890"}},
		existing, map[string]bool{"KEY-A": true}, fixedKeyMinter("new-"),
	)
	if len(repersist.Attachments) != 1 || repersist.Attachments[0].ObservedAt != nil {
		t.Fatalf("una ri-persistenza non porta un istante di osservazione: %+v", repersist.Attachments)
	}

	fromVendor := planMACompanyResolution(
		[]maCompanyObservation{{PriorCompanyKey: "KEY-A", VATCode: "01234567890", ObservedAt: &observedAt}},
		existing, map[string]bool{"KEY-A": true}, fixedKeyMinter("new-"),
	)
	if len(fromVendor.Attachments) != 1 || fromVendor.Attachments[0].ObservedAt == nil {
		t.Fatalf("un'osservazione vendor deve datare l'attach: %+v", fromVendor.Attachments)
	}
	if !fromVendor.Attachments[0].ObservedAt.Equal(observedAt) {
		t.Fatalf("istante = %s, atteso %s", fromVendor.Attachments[0].ObservedAt, observedAt)
	}
}

func TestPlanMACompanyResolutionNameOnlyOnVendorObservation(t *testing.T) {
	existing := map[maCompanyIdentifierRef]string{
		{maIdentifierNamespaceFiscal, "01234567890"}: "KEY-A",
	}
	observedAt := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)

	fromDB := planMACompanyResolution(
		[]maCompanyObservation{{VATCode: "01234567890", CompanyName: "Acme"}},
		existing, map[string]bool{}, fixedKeyMinter("new-"),
	)
	if len(fromDB.Names) != 0 {
		t.Fatalf("una rilettura dal DB non deve avanzare l'osservazione del nome: %+v", fromDB.Names)
	}

	fromVendor := planMACompanyResolution(
		[]maCompanyObservation{{VATCode: "01234567890", CompanyName: "Acme", ObservedAt: &observedAt, Source: maCompanyNameSourceVendor}},
		existing, map[string]bool{}, fixedKeyMinter("new-"),
	)
	if len(fromVendor.Names) != 1 || !fromVendor.Names[0].ObservedAt.Equal(observedAt) {
		t.Fatalf("osservazione del nome attesa dalla chiamata al fornitore: %+v", fromVendor.Names)
	}
}
