package hubspot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteNoteAndFile(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	if err := client.DeleteNote(context.Background(), 456); err != nil {
		t.Fatalf("DeleteNote() error = %v", err)
	}
	if err := client.DeleteFile(context.Background(), "file-123"); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}

	if got, want := strings.Join(paths, ","), "/crm/v3/objects/notes/456,/files/v3/files/file-123"; got != want {
		t.Fatalf("paths = %s, want %s", got, want)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(&APIError{StatusCode: http.StatusNotFound}) {
		t.Fatal("IsNotFound returned false for 404 API error")
	}
	if IsNotFound(&APIError{StatusCode: http.StatusInternalServerError}) {
		t.Fatal("IsNotFound returned true for 500 API error")
	}
}
