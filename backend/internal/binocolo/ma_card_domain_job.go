package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type maCardDomainVerifyPayload struct {
	InitiativeID string `json:"initiativeId"`
	CompanyKey   string `json:"companyKey"`
	VATCode      string `json:"vatCode,omitempty"`
	TaxCode      string `json:"taxCode,omitempty"`
	CompanyName  string `json:"companyName"`
	Domain       string `json:"domain"`
}

func (s *maService) runCardDomainVerifyJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation: "ma_card_domain_verify", CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail: job.CreatedByEmail, Request: job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.cardDomainVerifyWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
	return trace.id, nil
}

func (s *maService) cardDomainVerifyWork(ctx context.Context, job maJob) error {
	var payload maCardDomainVerifyPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode card domain verify payload: %w", err)
	}
	payload.InitiativeID = strings.TrimSpace(payload.InitiativeID)
	payload.CompanyKey = normalizeMACompanyKey(payload.CompanyKey)
	domain, ok := normalizeDomain(payload.Domain)
	if payload.InitiativeID == "" || payload.CompanyKey == "" || !ok {
		return fmt.Errorf("%w: direct card domain payload", errMAStrategyInvalid)
	}

	target := MATarget{
		CompanyKey: payload.CompanyKey, CompanyName: strings.TrimSpace(payload.CompanyName),
		VATCode: normalizeMAVATOrTax(payload.VATCode), TaxCode: normalizeMAVATOrTax(payload.TaxCode),
	}
	record := maCompanyDomain{
		CompanyKey: payload.CompanyKey, VATCode: target.VATCode, TaxCode: target.TaxCode,
		CompanyName: target.CompanyName, Domain: domain, Method: maDomainMethodManual,
		IdentityState: maIdentityStateVouched, CreatedBySubject: job.CreatedBySubject, CreatedByEmail: job.CreatedByEmail,
	}
	// Persist the analyst's authoritative association before the best-effort scrape.
	if err := s.store.UpsertMACompanyDomain(ctx, record); err != nil {
		return err
	}

	verified := false
	verificationAvailable := false
	pages := []string{}
	if s.scrape != nil {
		if result, err := s.scrape.ScrapeFull(ctx, "https://"+domain); err == nil && (result.StatusCode == 0 || (result.StatusCode >= 200 && result.StatusCode < 400)) {
			verificationAvailable = true
			markdown := strings.TrimSpace(result.Markdown)
			pages, verified = s.crawlResolvedSite(ctx, domain, markdown, target)
		}
		if !verified {
			verified, _ = s.deepVerifyIdentity(ctx, domain, target.VATCode, target.TaxCode)
		}
	}
	if verified {
		record.IdentityState = maIdentityStateVerified
		if err := s.store.UpsertMACompanyDomain(ctx, record); err != nil {
			return err
		}
	}

	if len(pages) > 0 {
		evidence, _ := s.gatherNeutralEvidence(ctx, domain, maWebValidationEvidenceCount, job.CreatedBySubject, job.CreatedByEmail, pages)
		if description := cleanText(s.representCompany(ctx, evidence, job.CreatedBySubject, job.CreatedByEmail), 850); description != "" {
			body := fmt.Sprintf("Descrizione dal sito (%s): %s", s.now().Format("2006-01-02"), description)
			registry, err := s.store.GetMACompanyRegistry(ctx, payload.CompanyKey)
			if err != nil {
				return err
			}
			if len(registry.Notes) == 0 || registry.Notes[0].Body != body {
				if _, err := s.addCompanyNote(ctx, payload.CompanyKey, body, job.CreatedBySubject, job.CreatedByEmail); err != nil {
					return err
				}
			}
		}
	}

	esito := "non_confermato"
	if verified {
		esito = "confermato"
	} else if !verificationAvailable {
		esito = "non_verificabile"
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID: payload.InitiativeID, CompanyKey: payload.CompanyKey, Event: maEventDominioVerificato,
		Payload:          maTraceJSON(map[string]any{"domain": domain, "esito": esito, "identityState": record.IdentityState}),
		CreatedBySubject: job.CreatedBySubject, CreatedByEmail: job.CreatedByEmail,
	}); err != nil {
		return err
	}
	return nil
}

func (s *maService) recordCardDomainUnverifiable(ctx context.Context, job maJob) {
	var payload maCardDomainVerifyPayload
	if json.Unmarshal(job.Payload, &payload) != nil || payload.InitiativeID == "" || payload.CompanyKey == "" {
		return
	}
	_ = s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID: payload.InitiativeID, CompanyKey: normalizeMACompanyKey(payload.CompanyKey), Event: maEventDominioVerificato,
		Payload:          maTraceJSON(map[string]any{"domain": payload.Domain, "esito": "non_verificabile", "identityState": maIdentityStateVouched}),
		CreatedBySubject: job.CreatedBySubject, CreatedByEmail: job.CreatedByEmail,
	})
}
