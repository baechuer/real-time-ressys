package event

import (
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type Clock interface{ Now() time.Time }

type Service struct {
	repo  EventRepo
	pub   EventPublisher // NEW
	clock Clock
}

func New(repo EventRepo, clock Clock, pub EventPublisher) *Service {
	return &Service{
		repo:  repo,
		pub:   pub,
		clock: clock,
	}
}

func isUser(role string) bool      { return role == "user" }
func isModerator(role string) bool { return role == "moderator" }
func isAdmin(role string) bool     { return role == "admin" }

// MVP: any authenticated user can create/manage own events.
// Moderator/Admin can manage others' events.
func canCreate(role string) bool {
	return isUser(role) || isModerator(role) || isAdmin(role)
}

func canEdit(ownerID, actorID, actorRole string) bool {
	if actorID == "" {
		return false
	}
	if actorID == ownerID {
		return true
	}
	return isModerator(actorRole) || isAdmin(actorRole)
}

var _ = domain.AppError{}

func canManage(actorID, actorRole, ownerID string) bool {
	if actorRole == "admin" {
		return true
	}
	return strings.TrimSpace(actorID) != "" && actorID == ownerID
}
