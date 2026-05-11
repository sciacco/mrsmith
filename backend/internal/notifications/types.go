package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/email"
)

const component = "notifications"

type Mailer interface {
	Enabled() bool
	Send(ctx context.Context, msg email.Message) error
}

type Notifier interface {
	Notify(ctx context.Context, input NotifyInput) (NotifyResult, error)
	Resolve(ctx context.Context, input ResolveInput) error
}

type Deps struct {
	DB     *sql.DB
	Store  Store
	Logger *slog.Logger
}

type NotificationType struct {
	TypeKey       string
	AppID         string
	TitleTemplate string
	BodyTemplate  string
	Severity      string
	DefaultPolicy json.RawMessage
	Enabled       bool
}

type Recipient struct {
	Subject string
	Email   string
	Name    string
}

type NotifyInput struct {
	TypeKey          string
	Title            string
	Body             string
	Severity         string
	EntityType       string
	EntityID         string
	DedupeKey        string
	DeepLink         string
	Metadata         map[string]any
	PolicyOverride   map[string]any
	Recipients       []Recipient
	CreatedBySubject string
	CreatedByEmail   string
}

type NotifyResult struct {
	NotificationID int64 `json:"notificationId"`
	RecipientCount int   `json:"recipientCount"`
	Created        bool  `json:"created"`
}

type ResolveInput struct {
	TypeKey    string
	EntityType string
	EntityID   string
	DedupeKey  string
}

type Summary struct {
	TotalUnread int64            `json:"totalUnread"`
	UnreadByApp map[string]int64 `json:"unreadByApp"`
}

type ListStatus string

const (
	ListStatusAll    ListStatus = "all"
	ListStatusUnread ListStatus = "unread"
)

type ListInput struct {
	Email           string
	Status          ListStatus
	AppID           string
	Limit           int
	CursorCreatedAt time.Time
	CursorID        int64
}

type NotificationItem struct {
	ID             int64           `json:"id"`
	NotificationID int64           `json:"notificationId"`
	TypeKey        string          `json:"typeKey"`
	AppID          string          `json:"appId"`
	Severity       string          `json:"severity"`
	Title          string          `json:"title"`
	Body           string          `json:"body"`
	EntityType     string          `json:"entityType"`
	EntityID       string          `json:"entityId"`
	DeepLink       string          `json:"deepLink"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
	ReadAt         *time.Time      `json:"readAt,omitempty"`
	ArchivedAt     *time.Time      `json:"archivedAt,omitempty"`
	ResolvedAt     *time.Time      `json:"resolvedAt,omitempty"`
}

type ListResult struct {
	Items         []NotificationItem
	NextCreatedAt time.Time
	NextID        int64
	HasNext       bool
}

type DeliverySpec struct {
	Channel    string
	PolicyStep string
	Status     string
	DueAt      time.Time
}

type CreateNotificationInput struct {
	TypeKey            string
	AppID              string
	Severity           string
	Title              string
	Body               string
	EntityType         string
	EntityID           string
	DedupeKey          string
	DeepLink           string
	MetadataJSON       []byte
	PolicyOverrideJSON []byte
	CreatedBySubject   string
	CreatedByEmail     string
	Recipients         []Recipient
	Deliveries         []DeliverySpec
}

type ClaimInput struct {
	WorkerID string
	Limit    int
	Now      time.Time
}

type ClaimedDelivery struct {
	ID             int64
	RecipientID    int64
	Channel        string
	PolicyStep     string
	AttemptCount   int
	DueAt          time.Time
	RecipientEmail string
	RecipientName  string
	ReadAt         *time.Time
	ArchivedAt     *time.Time
	ResolvedAt     *time.Time
	NotificationID int64
	TypeKey        string
	AppID          string
	Severity       string
	Title          string
	Body           string
	DeepLink       string
	CreatedAt      time.Time
}

type DeliveryCompletion struct {
	DeliveryID int64
	Status     string
	Error      string
	RetryAt    *time.Time
}

type Store interface {
	GetType(ctx context.Context, typeKey string) (NotificationType, error)
	CreateNotification(ctx context.Context, input CreateNotificationInput) (NotifyResult, error)
	Summary(ctx context.Context, email string) (Summary, error)
	List(ctx context.Context, input ListInput) (ListResult, error)
	MarkRead(ctx context.Context, email string, recipientID int64) (bool, error)
	MarkAllRead(ctx context.Context, email string) (int64, error)
	Archive(ctx context.Context, email string, recipientID int64) (bool, error)
	Resolve(ctx context.Context, input ResolveInput) (int64, error)
	ClaimDueDeliveries(ctx context.Context, input ClaimInput) ([]ClaimedDelivery, error)
	CompleteDelivery(ctx context.Context, completion DeliveryCompletion) error
}
