package models

import (
	"time"

	"github.com/google/uuid"
)

// Review represents a user review of a POI
type Review struct {
	Base
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	POIID       uuid.UUID  `json:"poi_id" db:"poi_id"`
	Rating      int        `json:"rating" db:"rating"`
	Title       string     `json:"title" db:"title"`
	Content     string     `json:"content" db:"content"`
	VisitDate   *time.Time `json:"visit_date" db:"visit_date"`
	ImageURLs   []string   `json:"image_urls" db:"image_urls,array"`
	Helpful     int        `json:"helpful" db:"helpful"`
	Unhelpful   int        `json:"unhelpful" db:"unhelpful"`
	IsVerified  bool       `json:"is_verified" db:"is_verified"`
	IsPublished bool       `json:"is_published" db:"is_published"`
	ModeratedAt *time.Time `json:"moderated_at" db:"moderated_at"`
}

// ReviewHelpful represents a user marking a review as helpful or unhelpful
type ReviewHelpful struct {
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	ReviewID  uuid.UUID `json:"review_id" db:"review_id"`
	IsHelpful bool      `json:"is_helpful" db:"is_helpful"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ReviewReply represents a reply to a review
type ReviewReply struct {
	Base
	ReviewID   uuid.UUID `json:"review_id" db:"review_id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Content    string    `json:"content" db:"content"`
	IsOfficial bool      `json:"is_official" db:"is_official"`
}

// NewReview creates a new review with default values
func NewReview(userID, poiID uuid.UUID, rating int, title, content string) *Review {
	return &Review{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		UserID:      userID,
		POIID:       poiID,
		Rating:      rating,
		Title:       title,
		Content:     content,
		Helpful:     0,
		Unhelpful:   0,
		IsVerified:  false,
		IsPublished: true,
	}
}

// NewReviewHelpful creates a new review helpful record
func NewReviewHelpful(userID, reviewID uuid.UUID, isHelpful bool) *ReviewHelpful {
	return &ReviewHelpful{
		UserID:    userID,
		ReviewID:  reviewID,
		IsHelpful: isHelpful,
		CreatedAt: time.Now(),
	}
}

// NewReviewReply creates a new review reply
func NewReviewReply(reviewID, userID uuid.UUID, content string, isOfficial bool) *ReviewReply {
	return &ReviewReply{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		ReviewID:   reviewID,
		UserID:     userID,
		Content:    content,
		IsOfficial: isOfficial,
	}
}