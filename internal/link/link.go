package link

import "context"

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusHidden   = "hidden"
)

// FriendLink is reserved. No HTTP route uses it in v1.
type FriendLink struct {
	ID          int64
	Name        string
	URL         string
	Description string
	Status      string
	SortOrder   int32
}

// Repository is the reserved data boundary. v1 does not implement it.
type Repository interface {
	List(ctx context.Context) ([]FriendLink, error)
}
