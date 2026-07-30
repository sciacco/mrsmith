package binocolo

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestMAAnnotationMutationQueriesGuardEventAndSoftDelete(t *testing.T) {
	for name, query := range map[string]string{
		"update": updateMAAnnotationQuery,
		"delete": softDeleteMAAnnotationQuery,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(query, "event = 'nota'") {
				t.Fatal("mutation query must guard event = 'nota'")
			}
			if !strings.Contains(query, "deleted_at IS NULL") {
				t.Fatal("mutation query must guard deleted_at IS NULL")
			}
		})
	}
}

func TestMACompanyActivityQuerySoftDeleteMode(t *testing.T) {
	if query := maCompanyActivityQuery(false); !strings.Contains(query, "AND o.deleted_at IS NULL") {
		t.Fatal("default activity query must exclude soft-deleted rows")
	}
	if query := maCompanyActivityQuery(true); strings.Contains(query, "AND o.deleted_at IS NULL") {
		t.Fatal("includeDeleted activity query must not filter soft-deleted rows")
	}
}

func TestMATargetOutcomeSelectsExcludeSoftDeleted(t *testing.T) {
	source, err := os.ReadFile("ma_store.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	needle := "FROM binocolo.ma_target_outcome"
	for offset := 0; ; {
		index := strings.Index(text[offset:], needle)
		if index < 0 {
			break
		}
		start := offset + index
		end := start + 500
		if end > len(text) {
			end = len(text)
		}
		window := text[start:end]
		if !strings.Contains(window, "deleted_at IS NULL") && !strings.Contains(window, "activity-includes-deleted") {
			t.Fatalf("ma_target_outcome SELECT near byte %d lacks a soft-delete predicate", start)
		}
		offset = start + len(needle)
	}
}

func TestMAAnnotationServiceValidationAndDomainErrors(t *testing.T) {
	store := &fakeMAWorkspaceStore{companyKnown: map[string]bool{"COMPANY-1": true}}
	service := newMAService(store, nil, nil, nil, nil, nil)

	annotation, err := service.createAnnotation(context.Background(), "company-1", MAAnnotationCreateRequest{Body: "  nota  valida "}, "subject", "agent@example.test")
	if err != nil {
		t.Fatalf("create annotation: %v", err)
	}
	if annotation.Note != "nota  valida" || annotation.SessionID != "" || len(store.outcomes) != 1 {
		t.Fatalf("unexpected annotation: %#v, stored=%d", annotation, len(store.outcomes))
	}
	if _, err := service.createAnnotation(context.Background(), "unknown", MAAnnotationCreateRequest{Body: "nota"}, "", ""); !errors.Is(err, errMACompanyKeyUnknown) {
		t.Fatalf("unknown company error = %v", err)
	}
	store.updateAnnotationErr = errMAAnnotationNotFound
	if err := service.updateAnnotation(context.Background(), annotation.ID, "testo", "", ""); !errors.Is(err, errMAAnnotationNotFound) {
		t.Fatalf("missing update error = %v", err)
	}
	store.deleteAnnotationErr = errMAAnnotationNotFound
	if err := service.deleteAnnotation(context.Background(), annotation.ID, "", ""); !errors.Is(err, errMAAnnotationNotFound) {
		t.Fatalf("missing delete error = %v", err)
	}
	status, code, _ := maHTTPError(errMAAnnotationNotFound)
	if status != http.StatusNotFound || code != "ma_annotation_not_found" {
		t.Fatalf("HTTP mapping = %d %q", status, code)
	}
}

func TestValidateMAAnnotationBody(t *testing.T) {
	body, err := validateMAAnnotationBody("  testo\n  annotazione  ")
	if err != nil || body != "testo\n  annotazione" {
		t.Fatalf("trimmed body = %q, err = %v", body, err)
	}
	if _, err := validateMAAnnotationBody(" \n "); err == nil {
		t.Fatal("empty body must be rejected")
	}
	longBody := strings.Repeat("è", 1001)
	if body, err := validateMAAnnotationBody(longBody); err != nil || body != longBody {
		t.Fatalf("long body = %q, err = %v", body, err)
	}
}
