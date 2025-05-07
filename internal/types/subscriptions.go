package types

import "context"

// SubscriptionRepository defines methods for accessing subscription data.
type SubscriptionRepository interface {
	// GetCurrentSubscriptionByUserID fetches the active/relevant subscription for a user.
	GetCurrentSubscriptionByUserID(ctx context.Context, userID string) (*Subscription, error)
	// CreateDefaultSubscription creates the initial (e.g., free) subscription for a new user.
	CreateDefaultSubscription(ctx context.Context, userID string) error
}

// Subscription holds basic plan and status information.
type Subscription struct {
	Plan   string `json:"plan"`   // e.g., "free", "premium_monthly".
	Status string `json:"status"` // e.g., "active", "past_due".
}
