package binocolo

import (
	"fmt"
	"strings"
	"time"
)

// Registro identità azienda (issue #86, migrazione 120).
//
// company_key è un identificatore NOSTRO, opaco: adottato dall'ObjectId
// OpenAPI.it per le aziende che il corpus già conosce, assegnato come UUID per
// quelle nuove. Non è più una derivazione del payload del vendor, e non va MAI
// ricalcolata da un payload esterno — è il punto dell'intera issue.
//
// Questo file contiene la parte PURA: estrazione degli identificatori e
// pianificazione della risoluzione di un batch. La parte transazionale sta in
// ma_company_identity_store.go.
//
// Distinzione portante: «la chiave» e «gli identificatori» non sono la stessa
// cosa. buildMAFiscalKey(vat, tax) risponde a «quale singolo valore chiava
// questo filing» e restituisce la P.IVA scartando un CF distinto — corretta per
// quel mestiere, e resta dov'è. Il registro ha bisogno di «quali valori
// identificano questa azienda», che è un INSIEME: scartare il CF perderebbe
// proprio l'identificatore su cui DocuEngine cerca i bilanci.

const (
	// maIdentifierNamespaceFiscal è UNIFICATO, non spaccato in vat/tax. Con
	// namespace separati la P.IVA dell'azienda A uguale al CF dell'azienda B
	// sarebbe silenziosamente permessa: la PK non vedrebbe collisione fra
	// namespace diversi. is_vat/is_tax restano come metadato di ruolo.
	maIdentifierNamespaceFiscal = "fiscal"
	maIdentifierNamespaceVendor = "vendor_openapiit"

	maCompanyIdentityStateFiscal     = "fiscal"
	maCompanyIdentityStateVendorOnly = "vendor_only"

	maCompanyOriginAdopted  = "adopted"
	maCompanyOriginAssigned = "assigned"

	maCompanyNameSourceVendor = "openapiit"
)

// maCompanyIdentifierRef è la chiave primaria di un identificatore.
type maCompanyIdentifierRef struct {
	Namespace string
	Value     string
}

// maCompanyIdentifier è un identificatore con i suoi ruoli. is_vat/is_tax
// valgono solo nel namespace fiscale e si aggiornano SEMPRE in OR: un ruolo già
// osservato non si perde perché un payload successivo non lo riporta.
type maCompanyIdentifier struct {
	maCompanyIdentifierRef
	IsVAT bool
	IsTax bool
}

// maCompanyObservation è un'occorrenza di azienda vista da un chiamante.
type maCompanyObservation struct {
	// PriorCompanyKey è la chiave che la riga PORTA GIÀ (re-persistenza di un
	// target riletto dal DB). Non è una derivazione: quando l'entità esiste è
	// l'azienda ATTESA, e un identificatore osservato che appartiene a
	// un'azienda diversa è un conflitto, non una correzione — re-chiavare la
	// riga lascerebbe il suo dettaglio sulla chiave vecchia.
	PriorCompanyKey string

	VendorID    string
	VATCode     string
	TaxCode     string
	CompanyName string

	// ObservedAt è l'istante della CHIAMATA AL FORNITORE, non della scrittura.
	// nil = la riga arriva dal nostro DB, quindi il nome non è una nuova
	// osservazione e non deve avanzare name_observed_at.
	ObservedAt *time.Time
	Source     string

	// Diagnostica dell'occorrenza, per il ledger dei conflitti. Nessuna FK.
	SessionID string
	RunID     string
	TargetID  string
}

// maCompanyIdentityConflict è una collisione: un identificatore rivendicato da
// due aziende diverse.
type maCompanyIdentityConflict struct {
	Namespace          string
	Value              string
	ExistingCompanyKey string
	IncomingCompanyKey string
	SessionID          string
	RunID              string
	TargetID           string
	CompanyName        string
}

// maCompanyIdentityConflictError è l'errore tipizzato del resolver. Porta
// TUTTI i conflitti del batch, non il primo: se serve una decisione umana,
// tanto vale darla completa.
type maCompanyIdentityConflictError struct {
	Conflicts []maCompanyIdentityConflict
}

func (e *maCompanyIdentityConflictError) Error() string {
	if e == nil || len(e.Conflicts) == 0 {
		return "ma company identity conflict"
	}
	parts := make([]string, 0, len(e.Conflicts))
	for _, conflict := range e.Conflicts {
		parts = append(parts, fmt.Sprintf("%s/%s: %s vs %s",
			conflict.Namespace, conflict.Value, conflict.ExistingCompanyKey, conflict.IncomingCompanyKey))
	}
	return "ma company identity conflict (" + strings.Join(parts, "; ") + ")"
}

// maCompanyIdentityMissingError è l'occorrenza priva di qualunque
// identificatore E di una chiave pregressa: non c'è nulla su cui fondare
// un'identità. Prima della 120 una riga così veniva chiavata sulla ragione
// sociale — cioè un'identità inventata da un nome. Qui erra invece di derivare.
type maCompanyIdentityMissingError struct {
	CompanyName string
	TargetID    string
}

func (e *maCompanyIdentityMissingError) Error() string {
	if e == nil {
		return "ma company identity missing"
	}
	return fmt.Sprintf("ma company identity missing: nessun identificatore (vendor id, P.IVA, codice fiscale) per %q", e.CompanyName)
}

// extractMACompanyIdentifiers restituisce l'INSIEME dei valori che identificano
// l'azienda:
//   - P.IVA e codice fiscale normalizzati SEPARATAMENTE con le primitive
//     (maStableVAT / normalizeMAFiscalValue), mai con il fallback composto;
//   - se coincidono, UNA riga fiscale che porta entrambi i ruoli;
//   - se differiscono, DUE righe nello stesso namespace fiscale;
//   - il vendor id nel proprio namespace, normalizzato con upper+trim, cioè
//     esattamente la forma con cui è stato adottato come company_key.
func extractMACompanyIdentifiers(vat, tax, vendorID string) []maCompanyIdentifier {
	out := make([]maCompanyIdentifier, 0, 3)
	vatValue := maStableVAT(vat)
	taxValue := normalizeMAFiscalValue(tax)
	switch {
	case vatValue != "" && vatValue == taxValue:
		out = append(out, maCompanyIdentifier{
			maCompanyIdentifierRef: maCompanyIdentifierRef{Namespace: maIdentifierNamespaceFiscal, Value: vatValue},
			IsVAT:                  true, IsTax: true,
		})
	default:
		if vatValue != "" {
			out = append(out, maCompanyIdentifier{
				maCompanyIdentifierRef: maCompanyIdentifierRef{Namespace: maIdentifierNamespaceFiscal, Value: vatValue},
				IsVAT:                  true,
			})
		}
		if taxValue != "" {
			out = append(out, maCompanyIdentifier{
				maCompanyIdentifierRef: maCompanyIdentifierRef{Namespace: maIdentifierNamespaceFiscal, Value: taxValue},
				IsTax:                  true,
			})
		}
	}
	if vendor := normalizeMACompanyKey(vendorID); vendor != "" {
		out = append(out, maCompanyIdentifier{
			maCompanyIdentifierRef: maCompanyIdentifierRef{Namespace: maIdentifierNamespaceVendor, Value: vendor},
		})
	}
	return out
}

// maCompanyEntityDraft è un'azienda da creare.
type maCompanyEntityDraft struct {
	CompanyKey    string
	CompanyName   string
	ObservedAt    *time.Time
	Source        string
	IdentityState string
}

// maCompanyIdentifierAttachment è un identificatore da collegare (o
// riconfermare) su un'azienda.
type maCompanyIdentifierAttachment struct {
	maCompanyIdentifier
	CompanyKey string
}

// maCompanyNameObservation è l'aggiornamento del nome, applicato solo quando
// l'osservazione è una vera chiamata al fornitore ed è più recente di quella
// già registrata.
type maCompanyNameObservation struct {
	CompanyKey  string
	CompanyName string
	ObservedAt  time.Time
	Source      string
}

// maCompanyResolutionPlan è l'esito della rilevazione: cosa scrivere, oppure
// perché non si scrive niente.
type maCompanyResolutionPlan struct {
	// Keys[i] è la company_key dell'osservazione i (vuota se in conflitto).
	Keys         []string
	NewEntities  []maCompanyEntityDraft
	Attachments  []maCompanyIdentifierAttachment
	Names        []maCompanyNameObservation
	Promotions   []string // chiavi da promuovere vendor_only -> fiscal
	Conflicts    []maCompanyIdentityConflict
	MissingIndex []int // osservazioni senza alcun identificatore né chiave pregressa
}

// planMACompanyResolution è il PRIMO dei due passaggi del resolver: normalizza e
// rileva TUTTI i conflitti — contro il DB e intra-batch — senza scrivere nulla.
//
// Non è una preferenza di stile ma un vincolo del motore: alla prima unique
// violation PostgreSQL manda la transazione in stato abortito e ogni comando
// successivo risponde «current transaction is aborted», quindi l'elenco completo
// dei conflitti sarebbe raccoglibile solo con un SAVEPOINT per riga.
//
// Cosa NON è un conflitto intra-batch, perché è facile implementarlo al
// contrario: due osservazioni dello stesso batch che mappano SULLA STESSA
// azienda non sono un conflitto, sono deduplica. Il conflitto è solo DUE
// AZIENDE DIVERSE che rivendicano lo stesso valore di identificatore.
//
// existing è la mappa (namespace, value) -> company_key già a DB; knownKeys sono
// le PriorCompanyKey che esistono davvero in ma_company; newKey conia la chiave
// di un'azienda nuova (UUID).
func planMACompanyResolution(
	observations []maCompanyObservation,
	existing map[maCompanyIdentifierRef]string,
	knownKeys map[string]bool,
	newKey func() string,
) maCompanyResolutionPlan {
	plan := maCompanyResolutionPlan{Keys: make([]string, len(observations))}

	// claimed parte dallo stato del DB e accumula le rivendicazioni del batch:
	// così una collisione intra-batch e una contro il DB seguono la stessa
	// regola, e due righe che mappano sulla stessa azienda si deduplicano.
	claimed := make(map[maCompanyIdentifierRef]string, len(existing)+len(observations)*2)
	for ref, key := range existing {
		claimed[ref] = key
	}
	promoted := map[string]bool{}
	created := map[string]bool{}

	for index, observation := range observations {
		identifiers := extractMACompanyIdentifiers(observation.VATCode, observation.TaxCode, observation.VendorID)
		prior := normalizeMACompanyKey(observation.PriorCompanyKey)

		if len(identifiers) == 0 && (prior == "" || !knownKeys[prior]) {
			plan.MissingIndex = append(plan.MissingIndex, index)
			continue
		}

		// La chiave che la riga PORTA GIÀ, quando l'entità esiste, è l'azienda
		// ATTESA: inizializza la risoluzione invece di essere un ripiego.
		//
		// Preferire gli identificatori l'avrebbe sostituita in silenzio, e una
		// riga ri-chiavata lascia indietro il proprio dettaglio — rating,
		// dossier, letture di tesi restano sulla chiave vecchia. È l'orfanamento
		// che questa issue esiste per impedire, quindi la divergenza fra chiave
		// portata e identificatori osservati è un CONFLITTO da decidere a mano,
		// non una riparazione automatica.
		key := ""
		if prior != "" && knownKeys[prior] {
			key = prior
		}
		conflicted := false
		for _, identifier := range identifiers {
			owner, ok := claimed[identifier.maCompanyIdentifierRef]
			if !ok {
				continue
			}
			if key == "" {
				key = owner
				continue
			}
			if owner != key {
				plan.Conflicts = append(plan.Conflicts, maCompanyIdentityConflict{
					Namespace:          identifier.Namespace,
					Value:              identifier.Value,
					ExistingCompanyKey: owner,
					IncomingCompanyKey: key,
					SessionID:          observation.SessionID,
					RunID:              observation.RunID,
					TargetID:           observation.TargetID,
					CompanyName:        observation.CompanyName,
				})
				conflicted = true
			}
		}
		if conflicted {
			continue
		}

		isNew := false
		if key == "" {
			key = newKey()
			isNew = true
		}
		plan.Keys[index] = key

		hasFiscal := false
		for _, identifier := range identifiers {
			if identifier.Namespace == maIdentifierNamespaceFiscal {
				hasFiscal = true
			}
		}

		if isNew && !created[key] {
			created[key] = true
			state := maCompanyIdentityStateVendorOnly
			if hasFiscal {
				state = maCompanyIdentityStateFiscal
			}
			plan.NewEntities = append(plan.NewEntities, maCompanyEntityDraft{
				CompanyKey:    key,
				CompanyName:   strings.TrimSpace(observation.CompanyName),
				ObservedAt:    observation.ObservedAt,
				Source:        observation.Source,
				IdentityState: state,
			})
		}

		// Attach-on-found: gli identificatori appena osservati si collegano
		// ANCHE quando l'azienda è già trovata. È il meccanismo con cui un
		// vendor id nuovo diventa storico senza rompere la continuità — la
		// funzione per cui questo lavoro esiste.
		for _, identifier := range identifiers {
			claimed[identifier.maCompanyIdentifierRef] = key
			plan.Attachments = append(plan.Attachments, maCompanyIdentifierAttachment{
				maCompanyIdentifier: identifier,
				CompanyKey:          key,
			})
		}

		if hasFiscal && !isNew && !promoted[key] {
			promoted[key] = true
			plan.Promotions = append(plan.Promotions, key)
		}

		if observation.ObservedAt != nil && !isNew && strings.TrimSpace(observation.CompanyName) != "" {
			plan.Names = append(plan.Names, maCompanyNameObservation{
				CompanyKey:  key,
				CompanyName: strings.TrimSpace(observation.CompanyName),
				ObservedAt:  *observation.ObservedAt,
				Source:      observation.Source,
			})
		}
	}
	return plan
}
