package support

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/email"
)

const (
	component = "support"

	emailNotificationSent    = "sent"
	emailNotificationSkipped = "skipped"
	emailNotificationFailed  = "failed"
	emailNotificationPending = "pending"

	configNamespaceSupport = "support"
	configKeyEmailTo       = "notification.email_to"
)

type Mailer interface {
	Enabled() bool
	Send(ctx context.Context, msg email.Message) error
}

type Store interface {
	CreateRequest(ctx context.Context, input CreateRequestInput) (int64, error)
	UpdateEmailStatus(ctx context.Context, id int64, status string, actor auth.Claims) error
	GetStringListConfig(ctx context.Context, namespace string, key string) ([]string, error)
}

type Deps struct {
	DB     *sql.DB
	Mailer Mailer
	Logger *slog.Logger
}

type CreateRequestInput struct {
	Priority                 string
	AppID                    string
	AppName                  string
	PageURL                  string
	PagePath                 string
	Message                  string
	Requester                auth.Claims
	TechnicalContextIncluded bool
	Context                  any
	Attachments              []CreateRequestAttachment
}

type CreateRequestAttachment struct {
	Filename      string
	ContentType   string
	SizeBytes     int64
	ContentSHA256 string
	Content       []byte
}

type createRequestPayload struct {
	Message                  string          `json:"message"`
	Priority                 string          `json:"priority"`
	TechnicalContextIncluded *bool           `json:"technicalContextIncluded"`
	Context                  json.RawMessage `json:"context"`
}

type createRequestResponse struct {
	ID                int64  `json:"id"`
	Status            string `json:"status"`
	EmailNotification string `json:"emailNotification"`
}
