package service

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/notifications/model"
	"restaurantsaas/internal/apps/notifications/repository"
)

type NotificationService interface {
	List(ctx context.Context, userID primitive.ObjectID) ([]*model.Notification, error)
	MarkRead(ctx context.Context, userID primitive.ObjectID, ids []primitive.ObjectID, all bool) error
	// Push writes a notification row and (eventually) fans out via FCM.
	// Today the FCM step is stubbed so push only persists. Best-effort:
	// a failure is logged but never propagated to the caller.
	Push(ctx context.Context, n *model.Notification)
}

type notificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) List(ctx context.Context, userID primitive.ObjectID) ([]*model.Notification, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *notificationService) MarkRead(ctx context.Context, userID primitive.ObjectID, ids []primitive.ObjectID, all bool) error {
	if all {
		return s.repo.MarkAllRead(ctx, userID)
	}
	return s.repo.MarkRead(ctx, userID, ids)
}

func (s *notificationService) Push(ctx context.Context, n *model.Notification) {
	if n == nil || n.UserID.IsZero() {
		return
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	if n.Color == "" {
		n.Color = model.ColorPrimary
	}
	if _, err := s.repo.Create(ctx, n); err != nil {
		log.Printf("notificationService.Push: insert: %v", err)
		return
	}
	// TODO: FCM fan-out via Firebase Admin SDK when FCM_CREDENTIALS_JSON
	// is set.  For now the device row collection is populated by the
	// devices app and consumers can read from there to drive their own
	// fan-out.
}
