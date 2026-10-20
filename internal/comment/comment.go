package comment

import "context"

// Status values match the comments.status check constraint.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusSpam     = "spam"
)

// Comment is reserved. No HTTP route uses it in v1.
type Comment struct {
	ID       int64
	PostID   int64
	UserID   *int64
	ParentID *int64
	Body     string
	Status   string
}

// Repository is the reserved data boundary. v1 does not implement it.
type Repository interface {
	ListByPost(ctx context.Context, postID int64) ([]Comment, error)
}
