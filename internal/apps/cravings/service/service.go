package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/cravings/model"
	"restaurantsaas/internal/apps/cravings/repository"
)

type CravingService interface {
	List(ctx context.Context, userID primitive.ObjectID) ([]*model.Craving, error)
	Pin(ctx context.Context, userID, id primitive.ObjectID, pinned bool) (*model.Craving, error)
	Delete(ctx context.Context, userID, id primitive.ObjectID) error
	// Record is called from aiService after a successful chat answer
	// to persist a session summary.  Best-effort: errors are returned
	// for the caller to log; the user-facing request never blocks on
	// this write.
	Record(ctx context.Context, userID primitive.ObjectID, title, summary, emoji string, match int) (*model.Craving, error)
}

type cravingService struct {
	repo *repository.CravingRepository
}

func NewCravingService(repo *repository.CravingRepository) CravingService {
	return &cravingService{repo: repo}
}

func (s *cravingService) List(ctx context.Context, userID primitive.ObjectID) ([]*model.Craving, error) {
	rows, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.DateLabel == "" {
			r.DateLabel = formatDateLabel(r.CreatedAt)
		}
	}
	return rows, nil
}

func (s *cravingService) Pin(ctx context.Context, userID, id primitive.ObjectID, pinned bool) (*model.Craving, error) {
	return s.repo.SetPinned(ctx, userID, id, pinned)
}

func (s *cravingService) Delete(ctx context.Context, userID, id primitive.ObjectID) error {
	return s.repo.DeleteForUser(ctx, userID, id)
}

func (s *cravingService) Record(ctx context.Context, userID primitive.ObjectID, title, summary, emoji string, match int) (*model.Craving, error) {
	now := time.Now().UTC()
	c := &model.Craving{
		UserID:    userID,
		Title:     title,
		Summary:   summary,
		Emoji:     emoji,
		Match:     match,
		Pinned:    false,
		DateLabel: formatDateLabel(now),
		CreatedAt: now,
	}
	return s.repo.InsertSafe(ctx, c)
}

// formatDateLabel mirrors the *_label convention from
// BACKEND_REQUIREMENTS.md §10 — when the timestamp is today we say
// "Today, 3:42 PM"; older days fall back to "Apr 24".
func formatDateLabel(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "Today, " + t.Format("3:04 PM")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}
