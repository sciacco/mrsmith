package training

import (
	"net/http"
	"strings"
	"testing"
)

func TestRegisterRoutesDoesNotConflict(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{})
}

func TestReadPDFUploadAcceptsOnlyPDFContent(t *testing.T) {
	content, mimeType, err := readPDFUpload(strings.NewReader("%PDF-1.4\nbody"), "application/pdf", 64)
	if err != nil {
		t.Fatalf("readPDFUpload accepted PDF: %v", err)
	}
	if string(content) != "%PDF-1.4\nbody" {
		t.Fatalf("content = %q", string(content))
	}
	if mimeType != "application/pdf" {
		t.Fatalf("mimeType = %q", mimeType)
	}

	_, _, err = readPDFUpload(strings.NewReader("plain text"), "text/plain", 64)
	if appErr, ok := asAppError(err); !ok || appErr.code != "unsupported_document_type" {
		t.Fatalf("plain text error = %#v, want unsupported_document_type", err)
	}

	_, _, err = readPDFUpload(strings.NewReader("plain text"), "application/pdf", 64)
	if appErr, ok := asAppError(err); !ok || appErr.code != "unsupported_document_type" {
		t.Fatalf("spoofed content-type error = %#v, want unsupported_document_type", err)
	}
}

func TestReadPDFUploadRejectsEmptyAndOversizeFiles(t *testing.T) {
	_, _, err := readPDFUpload(strings.NewReader(""), "application/pdf", 64)
	if appErr, ok := asAppError(err); !ok || appErr.code != "empty_document" {
		t.Fatalf("empty error = %#v, want empty_document", err)
	}

	_, _, err = readPDFUpload(strings.NewReader("%PDF-1.4"), "application/pdf", 4)
	if appErr, ok := asAppError(err); !ok || appErr.code != "document_too_large" {
		t.Fatalf("oversize error = %#v, want document_too_large", err)
	}
}
