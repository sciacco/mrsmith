package raenad

import (
	"context"
	"strings"

	"github.com/sciacco/mrsmith/internal/auth"
)

type actor struct {
	Subject string `json:"subject"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

func actorFromContext(ctx context.Context) (actor, bool) {
	claims, ok := auth.GetClaims(ctx)
	if !ok {
		return actor{}, false
	}
	return actor{
		Subject: strings.TrimSpace(claims.Subject),
		Email:   strings.TrimSpace(claims.Email),
		Name:    strings.TrimSpace(claims.Name),
	}, true
}
