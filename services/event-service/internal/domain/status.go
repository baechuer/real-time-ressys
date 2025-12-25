package domain

type EventStatus string

const (
	StatusDraft     EventStatus = "draft"
	StatusPublished EventStatus = "published"
	StatusCanceled  EventStatus = "canceled"
)

func (s EventStatus) Valid() bool {
	return s == StatusDraft || s == StatusPublished || s == StatusCanceled
}
