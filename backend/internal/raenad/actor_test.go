package raenad

import (
	"context"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestActorFromContextNormalizesClaims(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.ClaimsKey, auth.Claims{
		Subject: "  subject-1  ",
		Email:   "  user@example.com  ",
		Name:    "  Aenad User  ",
		Roles:   []string{"app_aenad_access"},
	})

	got, ok := actorFromContext(ctx)
	if !ok {
		t.Fatalf("expected actor")
	}
	if got.Subject != "subject-1" || got.Email != "user@example.com" || got.Name != "Aenad User" {
		t.Fatalf("unexpected actor: %#v", got)
	}
}

func TestActorFromContextMissingClaims(t *testing.T) {
	got, ok := actorFromContext(context.Background())
	if ok {
		t.Fatalf("expected missing actor, got %#v", got)
	}
}
