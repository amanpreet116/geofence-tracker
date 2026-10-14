package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// ConfigureAlertInput is the raw POST /alerts/configure payload. VehicleID
// nil or blank means the rule applies to all vehicles.
type ConfigureAlertInput struct {
	GeofenceID string
	VehicleID  *string
	EventType  string
}

// ValidateConfigureAlert checks field shapes; existence of the geofence and
// vehicle is checked by the database (foreign keys) in the same INSERT.
func ValidateConfigureAlert(in ConfigureAlertInput) (model.AlertRule, error) {
	errs := fieldErrors{}
	rule := model.AlertRule{
		GeofenceID: strings.TrimSpace(in.GeofenceID),
		EventType:  model.AlertEventType(strings.TrimSpace(in.EventType)),
		Status:     model.AlertStatusActive,
	}
	if rule.GeofenceID == "" {
		errs.add("geofence_id", "is required")
	}
	if in.VehicleID != nil {
		if id := strings.TrimSpace(*in.VehicleID); id != "" {
			rule.VehicleID = &id
		}
	}
	switch {
	case rule.EventType == "":
		errs.add("event_type", "is required")
	case !rule.EventType.Valid():
		errs.add("event_type", "must be one of entry, exit, both")
	}
	if err := errs.err("invalid alert rule"); err != nil {
		return model.AlertRule{}, err
	}
	return rule, nil
}

// ValidateAlertStatus checks the PATCH /alerts/{id} status value.
func ValidateAlertStatus(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	switch s {
	case model.AlertStatusActive, model.AlertStatusInactive:
		return s, nil
	case "":
		return "", fieldErrors{"status": "is required"}.err("invalid alert update")
	default:
		return "", fieldErrors{"status": "must be one of active, inactive"}.err("invalid alert update")
	}
}

// RuleMatches reports whether rule fires for event: same geofence, rule
// targets all vehicles or this vehicle, event type matches (or rule is
// "both"), and the rule is active.
func RuleMatches(rule model.AlertRule, event model.GeofenceEvent) bool {
	return rule.Status == model.AlertStatusActive &&
		rule.GeofenceID == event.Geofence.ID &&
		(rule.VehicleID == nil || *rule.VehicleID == event.VehicleID) &&
		(rule.EventType == model.AlertOnBoth || string(rule.EventType) == string(event.EventType))
}

// MatchRules pairs each event with the rules it triggers, keeping event
// order. Events that trigger nothing are omitted.
func MatchRules(rules []model.AlertRule, events []model.GeofenceEvent) []model.AlertMatch {
	var matches []model.AlertMatch
	for _, e := range events {
		var hit []model.AlertRule
		for _, r := range rules {
			if RuleMatches(r, e) {
				hit = append(hit, r)
			}
		}
		if len(hit) > 0 {
			matches = append(matches, model.AlertMatch{Event: e, Rules: hit})
		}
	}
	return matches
}

// AlertFilter narrows GET /alerts; empty fields mean "any".
type AlertFilter struct {
	GeofenceID string
	VehicleID  string
}

// AlertRepository is the persistence the alert service needs.
type AlertRepository interface {
	// CreateRule returns ErrNotFound for an unknown geofence/vehicle and
	// ErrConflict for an identical active rule.
	CreateRule(ctx context.Context, rule *model.AlertRule) error
	ListRules(ctx context.Context, f AlertFilter) ([]model.AlertRule, error)
	SetRuleStatus(ctx context.Context, id, status string) (model.AlertRule, error)
	DeleteRule(ctx context.Context, id string) error
	// CandidateRules returns, in one query, every active rule that could
	// match any of events (a superset; refine with MatchRules).
	CandidateRules(ctx context.Context, events []model.GeofenceEvent) ([]model.AlertRule, error)
}

// NotificationLister reads dispatched alerts for the history feed.
type NotificationLister interface {
	ListPayloads(ctx context.Context, limit int) ([]model.AlertPayload, error)
}

type AlertService struct {
	repo          AlertRepository
	notifications NotificationLister
}

func NewAlertService(repo AlertRepository, notifications NotificationLister) *AlertService {
	return &AlertService{repo: repo, notifications: notifications}
}

const (
	defaultNotificationLimit = 50
	maxNotificationLimit     = 200
)

// ParseNotificationLimit validates ?limit= for GET /alerts/notifications.
func ParseNotificationLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultNotificationLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxNotificationLimit {
		return 0, fieldErrors{"limit": fmt.Sprintf("must be an integer between 1 and %d", maxNotificationLimit)}.err("invalid query parameter")
	}
	return n, nil
}

// RecentNotifications returns the latest dispatched alert payloads, newest
// first, exactly as they were broadcast.
func (s *AlertService) RecentNotifications(ctx context.Context, rawLimit string) ([]model.AlertPayload, error) {
	limit, err := ParseNotificationLimit(rawLimit)
	if err != nil {
		return nil, err
	}
	payloads, err := s.notifications.ListPayloads(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return payloads, nil
}

func (s *AlertService) Configure(ctx context.Context, in ConfigureAlertInput) (model.AlertRule, error) {
	rule, err := ValidateConfigureAlert(in)
	if err != nil {
		return model.AlertRule{}, err
	}
	rule.ID = model.NewID(model.PrefixAlert)
	if err := s.repo.CreateRule(ctx, &rule); err != nil {
		return model.AlertRule{}, fmt.Errorf("configure alert: %w", err)
	}
	return rule, nil
}

func (s *AlertService) List(ctx context.Context, f AlertFilter) ([]model.AlertRule, error) {
	f.GeofenceID, f.VehicleID = strings.TrimSpace(f.GeofenceID), strings.TrimSpace(f.VehicleID)
	rules, err := s.repo.ListRules(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	return rules, nil
}

func (s *AlertService) SetStatus(ctx context.Context, id, rawStatus string) (model.AlertRule, error) {
	status, err := ValidateAlertStatus(rawStatus)
	if err != nil {
		return model.AlertRule{}, err
	}
	rule, err := s.repo.SetRuleStatus(ctx, id, status)
	if err != nil {
		return model.AlertRule{}, fmt.Errorf("update alert: %w", err)
	}
	return rule, nil
}

func (s *AlertService) Delete(ctx context.Context, id string) error {
	if err := s.repo.DeleteRule(ctx, id); err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}
	return nil
}

// Match returns the events that trigger at least one active rule, with
// those rules. One query fetches candidates for the whole batch.
func (s *AlertService) Match(ctx context.Context, events []model.GeofenceEvent) ([]model.AlertMatch, error) {
	if len(events) == 0 {
		return nil, nil
	}
	rules, err := s.repo.CandidateRules(ctx, events)
	if err != nil {
		return nil, fmt.Errorf("match alert rules: %w", err)
	}
	return MatchRules(rules, events), nil
}
