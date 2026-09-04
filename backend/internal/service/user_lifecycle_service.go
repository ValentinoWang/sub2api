package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Lifecycle email events. All are optional (users can unsubscribe) and sent at most once per user.
const (
	NotificationEmailEventUserWelcome  = "user.welcome"
	NotificationEmailEventUserInactive = "user.inactive_reminder"
	NotificationEmailEventUserWinback  = "user.winback"
)

const (
	lifecycleWelcomeWindow   = 48 * time.Hour
	lifecycleInactiveAfter   = 7 * 24 * time.Hour
	lifecycleWinbackAfter    = 30 * 24 * time.Hour
	lifecycleBatchLimit      = 200
	lifecycleDefaultInterval = 30 * time.Minute
)

// LifecycleCandidate is a user eligible for one lifecycle email.
type LifecycleCandidate struct {
	UserID       int64
	Email        string
	Username     string
	CreatedAt    time.Time
	LastActiveAt *time.Time
}

// UserLifecycleRepository persists the "already sent" ledger and finds candidates.
type UserLifecycleRepository interface {
	// ListWelcomeCandidates returns active users created after `since` who have not received `event`.
	ListWelcomeCandidates(ctx context.Context, event string, since time.Time, limit int) ([]LifecycleCandidate, error)
	// ListInactiveCandidates returns active users created before `createdBefore` whose last API
	// activity (or registration, if never active) is before `inactiveBefore` and who have not received `event`.
	ListInactiveCandidates(ctx context.Context, event string, createdBefore, inactiveBefore time.Time, limit int) ([]LifecycleCandidate, error)
	MarkSent(ctx context.Context, userID int64, event string) error
}

// UserLifecycleService sends welcome, inactivity and win-back emails on a ticker. It is a no-op
// unless the lifecycle_emails_enabled setting is on and SMTP is configured.
type UserLifecycleService struct {
	repo                     UserLifecycleRepository
	notificationEmailService *NotificationEmailService
	settingService           *SettingService
	interval                 time.Duration
	now                      func() time.Time

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewUserLifecycleService(
	repo UserLifecycleRepository,
	notificationEmailService *NotificationEmailService,
	settingService *SettingService,
	interval time.Duration,
) *UserLifecycleService {
	if interval <= 0 {
		interval = lifecycleDefaultInterval
	}
	return &UserLifecycleService{
		repo:                     repo,
		notificationEmailService: notificationEmailService,
		settingService:           settingService,
		interval:                 interval,
		now:                      time.Now,
		stopCh:                   make(chan struct{}),
	}
}

func (s *UserLifecycleService) Start() {
	if s == nil || s.repo == nil || s.notificationEmailService == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		// Give the process a moment to finish booting before the first pass.
		select {
		case <-time.After(2 * time.Minute):
		case <-s.stopCh:
			return
		}
		s.RunOnce(context.Background())
		for {
			select {
			case <-ticker.C:
				s.RunOnce(context.Background())
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *UserLifecycleService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

// RunOnce performs a single pass over the three lifecycle events.
func (s *UserLifecycleService) RunOnce(ctx context.Context) {
	if s == nil || s.repo == nil || s.notificationEmailService == nil {
		return
	}
	if s.settingService != nil && !s.settingService.IsLifecycleEmailsEnabled(ctx) {
		return
	}
	now := s.now()
	s.runWelcome(ctx, now)
	s.runInactive(ctx, now, NotificationEmailEventUserInactive, lifecycleInactiveAfter)
	s.runInactive(ctx, now, NotificationEmailEventUserWinback, lifecycleWinbackAfter)
}

func (s *UserLifecycleService) runWelcome(ctx context.Context, now time.Time) {
	candidates, err := s.repo.ListWelcomeCandidates(ctx, NotificationEmailEventUserWelcome, now.Add(-lifecycleWelcomeWindow), lifecycleBatchLimit)
	if err != nil {
		slog.Warn("user_lifecycle: list welcome candidates failed", "error", err)
		return
	}
	for _, c := range candidates {
		s.deliver(ctx, c, NotificationEmailEventUserWelcome, map[string]string{})
	}
}

func (s *UserLifecycleService) runInactive(ctx context.Context, now time.Time, event string, after time.Duration) {
	cutoff := now.Add(-after)
	candidates, err := s.repo.ListInactiveCandidates(ctx, event, cutoff, cutoff, lifecycleBatchLimit)
	if err != nil {
		slog.Warn("user_lifecycle: list inactive candidates failed", "event", event, "error", err)
		return
	}
	days := strconv.Itoa(int(after.Hours() / 24))
	for _, c := range candidates {
		s.deliver(ctx, c, event, map[string]string{"days_inactive": days})
	}
}

func (s *UserLifecycleService) deliver(ctx context.Context, c LifecycleCandidate, event string, vars map[string]string) {
	email := strings.TrimSpace(c.Email)
	if email == "" || isReservedEmail(email) {
		// Synthetic OAuth mailboxes cannot receive mail; record so we do not retry forever.
		_ = s.repo.MarkSent(ctx, c.UserID, event)
		return
	}
	name := strings.TrimSpace(c.Username)
	if name == "" {
		name = email
	}
	if vars == nil {
		vars = map[string]string{}
	}
	vars["site_url"] = s.siteURL(ctx)
	sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err := s.notificationEmailService.Send(sendCtx, NotificationEmailSendInput{
		Event:          event,
		RecipientEmail: email,
		RecipientName:  name,
		UserID:         c.UserID,
		SourceType:     "lifecycle",
		SourceID:       strconv.FormatInt(c.UserID, 10),
		ReminderKey:    event,
		Variables:      vars,
	})
	cancel()
	if err != nil {
		slog.Warn("user_lifecycle: send failed", "event", event, "user_id", c.UserID, "error", err)
		return
	}
	if err := s.repo.MarkSent(ctx, c.UserID, event); err != nil {
		slog.Warn("user_lifecycle: mark sent failed", "event", event, "user_id", c.UserID, "error", err)
	}
}

func (s *UserLifecycleService) siteURL(ctx context.Context) string {
	if s.settingService == nil {
		return ""
	}
	settings, err := s.settingService.GetPublicSettings(ctx)
	if err != nil || settings == nil {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(settings.APIBaseURL), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/login", base)
}
