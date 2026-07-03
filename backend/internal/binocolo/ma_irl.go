package binocolo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Fase 6 (filone F): Information Request List per (iniziativa, azienda) — flag
// e domande diventano l'artefatto che esce dal tool. Il seed assembla dalle 4
// fonti con provenienza (flag deterministici, brief neutro, lettura di tesi,
// template della famiglia ratificata); dopo il seed la lista è dell'analista:
// il re-seed è SOLO additivo (source_ref nuovi) e non tocca mai la curatela.

var maIRLStatuses = map[string]bool{
	"aperta":   true,
	"chiesta":  true,
	"risposta": true,
	"na":       true,
}

// maIRLFlagCategories mappa i codici flag qualità (Fase 2/4) sulla categoria
// IRL; default finanziaria (i flag nascono quasi tutti dal CE/SP).
var maIRLFlagCategories = map[string]string{
	"b8_beni_terzi":          "legale",
	"perimetro_standalone":   "legale",
	"deriva_valore_aggiunto": "commerciale",
}

// maIRLSourceRef produce la chiave stabile del re-seed additivo per le domande
// testuali (brief/tesi): hash del testo normalizzato, insensibile a spazi e
// maiuscole ma sensibile alla riformulazione (una domanda riscritta dal
// modello è a tutti gli effetti una proposta nuova).
func maIRLSourceRef(question string) string {
	norm := strings.ToLower(strings.Join(strings.Fields(question), " "))
	sum := sha1.Sum([]byte(norm))
	return hex.EncodeToString(sum[:])[:16]
}

// getCardIRL lista le voci della card. Sola esistenza della card, nessun gate
// operativo: l'IRL sopravvive all'archiviazione (memoria istituzionale).
func (s *maService) getCardIRL(ctx context.Context, initiativeID, companyKey string) ([]MACardIRLItem, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.store.GetMAInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, errMACardNotFound
	}
	return s.store.ListMACardIRLItems(ctx, initiativeID, companyKey)
}

// seedCardIRL assembla le proposte dalle 4 fonti e inserisce SOLO i source_ref
// nuovi (ON CONFLICT DO NOTHING sull'indice parziale): le voci esistenti —
// curate, ristatate, ricategorizzate — non vengono mai toccate.
func (s *maService) seedCardIRL(ctx context.Context, initiativeID, companyKey, email string) (MAIRLSeedReport, error) {
	report := MAIRLSeedReport{BySource: map[string]int{}}
	if s.store == nil {
		return report, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return report, err
	}
	proposals := []MACardIRLItem{}
	add := func(source, sourceRef, category, question string) {
		question = strings.TrimSpace(question)
		if question == "" {
			return
		}
		proposals = append(proposals, MACardIRLItem{
			InitiativeID: initiativeID,
			CompanyKey:   card.CompanyKey,
			Category:     category,
			Question:     question,
			Source:       source,
			SourceRef:    sourceRef,
			Status:       "aperta",
		})
	}

	// 1+2) Dossier deep: flag deterministici (ref = codice flag) + domande DD
	// del brief neutro (ref = hash della domanda). Senza deep pronto le fonti
	// restanti bastano: il seed non lo richiede.
	if deep, err := s.store.ListMADeepAnalysis(ctx, []string{card.CompanyKey}); err == nil {
		if record, ok := deep[card.CompanyKey]; ok && record.Status == maDeepStatusReady {
			if record.Scorecard != nil {
				for _, flag := range record.Scorecard.QualityFlags {
					category := maIRLFlagCategories[flag.Code]
					if category == "" {
						category = "finanziaria"
					}
					question := flag.DDQuestion
					if question == "" {
						question = flag.Label + ": documentare e quantificare. (" + flag.Evidence + ")"
					}
					add("flag", flag.Code, category, question)
				}
			}
			if record.Brief != nil {
				for _, question := range record.Brief.DDQuestions {
					add("brief", maIRLSourceRef(question), "generale", question)
				}
			}
		}
	}

	// 3) Lettura di tesi, se generata (ref = hash della domanda).
	if reading, err := s.store.GetMACardThesisReading(ctx, initiativeID, card.CompanyKey); err == nil && reading != nil {
		for _, question := range reading.Reading.ThesisDDQuestions {
			add("thesis", maIRLSourceRef(question), "tesi", question)
		}
	}

	// 4) Template della famiglia di business model (ratifica > suggerimento);
	// senza famiglia si semina dalle sole fonti restanti.
	if family, err := s.store.GetMABMFamily(ctx, card.CompanyKey); err == nil && family != nil {
		if effective, _ := family.Effective(); effective != "" {
			templates, err := s.store.ListMAIRLTemplates(ctx, effective)
			if err != nil {
				return report, err
			}
			for _, t := range templates {
				add("template", t.ID, t.Category, t.Question)
			}
		}
	}

	report.Proposed = len(proposals)
	if len(proposals) == 0 {
		return report, nil
	}
	basePosition, err := s.store.MaxMACardIRLPosition(ctx, initiativeID, card.CompanyKey)
	if err != nil {
		return report, err
	}
	for i := range proposals {
		proposals[i].Position = basePosition + (i+1)*10
		proposals[i].CreatedByEmail = email
	}
	inserted, bySource, err := s.store.InsertMACardIRLSeed(ctx, proposals)
	if err != nil {
		return report, err
	}
	report.Inserted = inserted
	report.BySource = bySource
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_irl_seeded",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"initiative_id": initiativeID, "company_key": card.CompanyKey, "proposed": report.Proposed, "inserted": inserted, "by_source": bySource, "by": email}),
	})
	return report, nil
}

// addCardIRLItem aggiunge una voce dell'analista (source=analyst, nessun
// source_ref: fuori dal lucchetto del re-seed).
func (s *maService) addCardIRLItem(ctx context.Context, initiativeID, companyKey, category, question, email string) (*MACardIRLItem, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return nil, err
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("%w: domanda vuota", errMAStrategyInvalid)
	}
	basePosition, err := s.store.MaxMACardIRLPosition(ctx, initiativeID, card.CompanyKey)
	if err != nil {
		return nil, err
	}
	item := MACardIRLItem{
		InitiativeID:   initiativeID,
		CompanyKey:     card.CompanyKey,
		Category:       strings.TrimSpace(category),
		Question:       question,
		Source:         "analyst",
		Status:         "aperta",
		Position:       basePosition + 10,
		CreatedByEmail: email,
	}
	saved, err := s.store.InsertMACardIRLItem(ctx, item)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// MAIRLItemPatch è la modifica parziale di una voce: i campi nil restano
// invariati (curatela incrementale: cambio stato, riformulazione, categoria).
type MAIRLItemPatch struct {
	Category *string `json:"category"`
	Question *string `json:"question"`
	Status   *string `json:"status"`
}

func (s *maService) updateCardIRLItem(ctx context.Context, initiativeID, companyKey, itemID string, patch MAIRLItemPatch) (*MACardIRLItem, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(itemID); err != nil {
		return nil, fmt.Errorf("%w: id voce IRL", errMAStrategyInvalid)
	}
	if patch.Status != nil && !maIRLStatuses[*patch.Status] {
		return nil, fmt.Errorf("%w: stato IRL sconosciuto", errMAStrategyInvalid)
	}
	if patch.Question != nil && strings.TrimSpace(*patch.Question) == "" {
		return nil, fmt.Errorf("%w: domanda vuota", errMAStrategyInvalid)
	}
	item, err := s.store.UpdateMACardIRLItem(ctx, initiativeID, card.CompanyKey, itemID, patch)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errMACardNotFound
	}
	return item, nil
}

func (s *maService) deleteCardIRLItem(ctx context.Context, initiativeID, companyKey, itemID string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(itemID); err != nil {
		return fmt.Errorf("%w: id voce IRL", errMAStrategyInvalid)
	}
	found, err := s.store.DeleteMACardIRLItem(ctx, initiativeID, card.CompanyKey, itemID)
	if err != nil {
		return err
	}
	if !found {
		return errMACardNotFound
	}
	return nil
}

func (s *maService) reorderCardIRL(ctx context.Context, initiativeID, companyKey string, itemIDs []string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return err
	}
	if len(itemIDs) == 0 {
		return fmt.Errorf("%w: nessuna voce da riordinare", errMAStrategyInvalid)
	}
	for _, itemID := range itemIDs {
		if _, err := uuid.Parse(itemID); err != nil {
			return fmt.Errorf("%w: id voce IRL", errMAStrategyInvalid)
		}
	}
	return s.store.ReorderMACardIRLItems(ctx, initiativeID, card.CompanyKey, itemIDs)
}

// exportCardIRL produce l'XLSX del kick-off DD (pattern exportSession). Sola
// esistenza della card: l'export deve funzionare anche a card archiviata.
func (s *maService) exportCardIRL(ctx context.Context, initiativeID, companyKey, email string) ([]byte, string, string, error) {
	if s.store == nil {
		return nil, "", "", errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.store.GetMAInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return nil, "", "", err
	}
	if card == nil {
		return nil, "", "", errMACardNotFound
	}
	items, err := s.store.ListMACardIRLItems(ctx, initiativeID, card.CompanyKey)
	if err != nil {
		return nil, "", "", err
	}
	sourceLabels := map[string]string{
		"flag":     "Flag dossier",
		"brief":    "Brief dossier",
		"thesis":   "Lettura di tesi",
		"template": "Template famiglia",
		"analyst":  "Analista",
	}
	rows := [][]any{{"Categoria", "Domanda", "Stato", "Fonte", "Aggiornata"}}
	for _, item := range items {
		source := sourceLabels[item.Source]
		if source == "" {
			source = item.Source
		}
		rows = append(rows, []any{item.Category, item.Question, item.Status, source, item.UpdatedAt.Format(time.DateOnly)})
	}
	content, err := buildMAXLSX(rows)
	if err != nil {
		return nil, "", "", err
	}
	name := card.CompanyName
	if name == "" {
		name = card.CompanyKey
	}
	filename := "irl-" + safeFilenamePart(name) + ".xlsx"
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_irl_exported",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"initiative_id": initiativeID, "company_key": card.CompanyKey, "rows": len(items), "by": email}),
	})
	return content, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}
