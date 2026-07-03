package hubspot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDeleteNoteAndFile(t *testing.T) {
	var paths []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", NewMockClient(handler))
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

func TestCreateGenericNoteWithAttachment(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/crm/v3/objects/notes"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var body struct {
			Properties   map[string]string   `json:"properties"`
			Associations []ObjectAssociation `json:"associations"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, want := body.Properties["hs_timestamp"], "2026-06-14T10:00:00Z"; got != want {
			t.Fatalf("timestamp = %s, want %s", got, want)
		}
		if got, want := body.Properties["hs_note_body"], "Quote PDF"; got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
		if got, want := body.Properties["hs_attachment_ids"], "file-1;file-2"; got != want {
			t.Fatalf("attachments = %s, want %s", got, want)
		}
		if got, want := body.Associations[0].To.ID, "deal-123"; got != want {
			t.Fatalf("association id = %s, want %s", got, want)
		}
		if got, want := body.Associations[0].Types[0].TypeID, AssocTypeNoteToDeal; got != want {
			t.Fatalf("association type = %d, want %d", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":987}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", NewMockClient(handler))
	noteID, err := client.CreateGenericNoteWithAttachment(context.Background(), NoteWithAttachmentRequest{
		TargetObjectType:  ObjectTypeDeal,
		TargetObjectID:    "deal-123",
		AssociationTypeID: AssocTypeNoteToDeal,
		AttachmentIDs:     []string{"file-1", "file-2"},
		Body:              "Quote PDF",
		Timestamp:         time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateGenericNoteWithAttachment() error = %v", err)
	}
	if got, want := noteID, "987"; got != want {
		t.Fatalf("noteID = %s, want %s", got, want)
	}
}

func TestCreateNoteWithAttachmentCompatibility(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/crm/v3/objects/notes"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var body struct {
			Properties   map[string]string   `json:"properties"`
			Associations []ObjectAssociation `json:"associations"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, want := body.Properties["hs_note_body"], "Order PDF 42"; got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
		if got, want := body.Properties["hs_attachment_ids"], "file-123"; got != want {
			t.Fatalf("attachment = %s, want %s", got, want)
		}
		if got, want := body.Associations[0].To.ID, "123"; got != want {
			t.Fatalf("association id = %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"456"}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", NewMockClient(handler))
	noteID, err := client.CreateNoteWithAttachment(context.Background(), "123", "file-123", 42)
	if err != nil {
		t.Fatalf("CreateNoteWithAttachment() error = %v", err)
	}
	if got, want := noteID, int64(456); got != want {
		t.Fatalf("noteID = %d, want %d", got, want)
	}
}

func TestCreateNoteWithAttachmentInvalidReturnedID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"note-456"}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", NewMockClient(handler))
	if _, err := client.CreateNoteWithAttachment(context.Background(), "123", "file-123", 42); err == nil {
		t.Fatal("CreateNoteWithAttachment() error = nil")
	}
}

func TestCreateGenericNoteWithAttachmentMissingReturnedID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"properties":{}}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", NewMockClient(handler))
	_, err := client.CreateGenericNoteWithAttachment(context.Background(), NoteWithAttachmentRequest{
		TargetObjectType:  ObjectTypeDeal,
		TargetObjectID:    "123",
		AssociationTypeID: AssocTypeNoteToDeal,
		AttachmentIDs:     []string{"file-123"},
	})
	if err == nil {
		t.Fatal("CreateGenericNoteWithAttachment() error = nil")
	}
}
