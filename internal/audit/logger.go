// Package audit provides audit logging capabilities for tracking
// deployment changes, configuration modifications, and system events.
package audit

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// EventType represents the type of audit event
type EventType string

const (
	// EventTypeDeployment is a deployment-related event
	EventTypeDeployment EventType = "deployment"
	// EventTypeConfiguration is a configuration change event
	EventTypeConfiguration EventType = "configuration"
	// EventTypeManifest is a manifest-related event
	EventTypeManifest EventType = "manifest"
	// EventTypeRollback is a rollback event
	EventTypeRollback EventType = "rollback"
	// EventTypeState is a state change event
	EventTypeState EventType = "state"
	// EventTypeError is an error event
	EventTypeError EventType = "error"
)

// EventAction represents the action taken
type EventAction string

const (
	// ActionCreate indicates a resource was created
	ActionCreate EventAction = "create"
	// ActionUpdate indicates a resource was updated
	ActionUpdate EventAction = "update"
	// ActionDelete indicates a resource was deleted
	ActionDelete EventAction = "delete"
	// ActionApply indicates a resource was applied
	ActionApply EventAction = "apply"
	// ActionRollback indicates a rollback was performed
	ActionRollback EventAction = "rollback"
	// ActionValidate indicates validation occurred
	ActionValidate EventAction = "validate"
	// ActionMigrate indicates a migration occurred
	ActionMigrate EventAction = "migrate"
)

// EventSeverity represents the severity of an event
type EventSeverity string

const (
	// SeverityInfo is informational
	SeverityInfo EventSeverity = "info"
	// SeverityWarning is a warning
	SeverityWarning EventSeverity = "warning"
	// SeverityError is an error
	SeverityError EventSeverity = "error"
	// SeverityCritical is critical
	SeverityCritical EventSeverity = "critical"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	// ID is the unique identifier for this event
	ID string `json:"id"`
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`
	// Type is the type of event
	Type EventType `json:"type"`
	// Action is what action was taken
	Action EventAction `json:"action"`
	// Severity indicates the importance of the event
	Severity EventSeverity `json:"severity"`
	// ResourceID identifies the affected resource
	ResourceID string `json:"resource_id"`
	// ResourceType is the type of resource affected
	ResourceType string `json:"resource_type"`
	// Actor is who or what triggered the event
	Actor string `json:"actor"`
	// Description is a human-readable description
	Description string `json:"description"`
	// OldValue is the previous value (for updates)
	OldValue interface{} `json:"old_value,omitempty"`
	// NewValue is the new value (for creates/updates)
	NewValue interface{} `json:"new_value,omitempty"`
	// Metadata holds additional context
	Metadata map[string]string `json:"metadata,omitempty"`
	// CorrelationID links related events
	CorrelationID string `json:"correlation_id,omitempty"`
	// ParentEventID links to a parent event
	ParentEventID string `json:"parent_event_id,omitempty"`
	// Duration is how long the action took
	Duration time.Duration `json:"duration,omitempty"`
	// Success indicates if the action succeeded
	Success bool `json:"success"`
	// ErrorMessage contains error details if failed
	ErrorMessage string `json:"error_message,omitempty"`
}

// AuditFilter defines criteria for filtering audit events
type AuditFilter struct {
	// Types filters by event types
	Types []EventType `json:"types,omitempty"`
	// Actions filters by actions
	Actions []EventAction `json:"actions,omitempty"`
	// Severities filters by severities
	Severities []EventSeverity `json:"severities,omitempty"`
	// ResourceID filters by resource ID
	ResourceID string `json:"resource_id,omitempty"`
	// ResourceType filters by resource type
	ResourceType string `json:"resource_type,omitempty"`
	// Actor filters by actor
	Actor string `json:"actor,omitempty"`
	// CorrelationID filters by correlation ID
	CorrelationID string `json:"correlation_id,omitempty"`
	// StartTime filters events after this time
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime filters events before this time
	EndTime *time.Time `json:"end_time,omitempty"`
	// SuccessOnly filters to only successful events
	SuccessOnly bool `json:"success_only,omitempty"`
	// FailedOnly filters to only failed events
	FailedOnly bool `json:"failed_only,omitempty"`
	// Limit restricts the number of results
	Limit int `json:"limit,omitempty"`
	// Offset skips this many results
	Offset int `json:"offset,omitempty"`
}

// AuditSummary provides statistics about audit events
type AuditSummary struct {
	// TotalEvents is the total number of events
	TotalEvents int `json:"total_events"`
	// EventsByType counts events by type
	EventsByType map[EventType]int `json:"events_by_type"`
	// EventsByAction counts events by action
	EventsByAction map[EventAction]int `json:"events_by_action"`
	// EventsBySeverity counts events by severity
	EventsBySeverity map[EventSeverity]int `json:"events_by_severity"`
	// SuccessCount is the number of successful events
	SuccessCount int `json:"success_count"`
	// FailureCount is the number of failed events
	FailureCount int `json:"failure_count"`
	// FirstEvent is the timestamp of the earliest event
	FirstEvent *time.Time `json:"first_event,omitempty"`
	// LastEvent is the timestamp of the most recent event
	LastEvent *time.Time `json:"last_event,omitempty"`
	// AverageDuration is the average event duration
	AverageDuration time.Duration `json:"average_duration"`
	// TopActors lists the most active actors
	TopActors []ActorStat `json:"top_actors,omitempty"`
	// TopResources lists the most affected resources
	TopResources []ResourceStat `json:"top_resources,omitempty"`
}

// ActorStat provides statistics for an actor
type ActorStat struct {
	Actor      string `json:"actor"`
	EventCount int    `json:"event_count"`
}

// ResourceStat provides statistics for a resource
type ResourceStat struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	EventCount   int    `json:"event_count"`
}

// AuditExport is the exportable format of audit logs
type AuditExport struct {
	// Version is the export format version
	Version string `json:"version"`
	// ExportedAt is when the export was created
	ExportedAt time.Time `json:"exported_at"`
	// Events is the list of audit events
	Events []AuditEvent `json:"events"`
	// Summary provides statistics
	Summary *AuditSummary `json:"summary,omitempty"`
}

// Logger is the interface for audit logging
type Logger interface {
	// Log records an audit event
	Log(event *AuditEvent) error
	// LogDeployment logs a deployment-related event
	LogDeployment(resourceID, actor, description string, action EventAction, success bool, metadata map[string]string) (*AuditEvent, error)
	// LogConfiguration logs a configuration change
	LogConfiguration(resourceID, actor string, oldValue, newValue interface{}, metadata map[string]string) (*AuditEvent, error)
	// LogManifest logs a manifest-related event
	LogManifest(resourceID, actor, description string, action EventAction, success bool, metadata map[string]string) (*AuditEvent, error)
	// LogRollback logs a rollback event
	LogRollback(resourceID, actor string, fromVersion, toVersion string, success bool, metadata map[string]string) (*AuditEvent, error)
	// LogError logs an error event
	LogError(resourceID, actor, errorMessage string, metadata map[string]string) (*AuditEvent, error)
	// Get retrieves an event by ID
	Get(id string) (*AuditEvent, bool)
	// List returns all events
	List() []AuditEvent
	// Query filters events based on criteria
	Query(filter *AuditFilter) []AuditEvent
	// GetByCorrelation returns events with the same correlation ID
	GetByCorrelation(correlationID string) []AuditEvent
	// GetSummary returns statistics about audit events
	GetSummary() *AuditSummary
	// Export exports audit logs
	Export() (*AuditExport, error)
	// Import imports audit logs
	Import(export *AuditExport) error
	// ToJSON serializes to JSON
	ToJSON() ([]byte, error)
	// FromJSON deserializes from JSON
	FromJSON(data []byte) error
	// Clear removes all events
	Clear()
	// Prune removes events older than the given time
	Prune(before time.Time) int
}

// InMemoryLogger is an in-memory implementation of the audit logger
type InMemoryLogger struct {
	mu       sync.RWMutex
	events   map[string]*AuditEvent
	eventSeq int64
	maxSize  int
}

// NewInMemoryLogger creates a new in-memory audit logger
func NewInMemoryLogger(maxSize int) *InMemoryLogger {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &InMemoryLogger{
		events:  make(map[string]*AuditEvent),
		maxSize: maxSize,
	}
}

// generateID creates a unique event ID
func (l *InMemoryLogger) generateID() string {
	l.eventSeq++
	return fmt.Sprintf("evt-%d-%d", time.Now().UnixNano(), l.eventSeq)
}

// Log records an audit event
func (l *InMemoryLogger) Log(event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	if event.ID == "" {
		event.ID = l.generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Prune oldest events if at capacity
	if len(l.events) >= l.maxSize {
		l.pruneOldest(l.maxSize / 10) // Remove 10% of oldest events
	}

	l.events[event.ID] = event
	return nil
}

// pruneOldest removes the oldest n events (must be called with lock held)
func (l *InMemoryLogger) pruneOldest(n int) {
	if n <= 0 || len(l.events) == 0 {
		return
	}

	// Get all events sorted by timestamp
	events := make([]*AuditEvent, 0, len(l.events))
	for _, e := range l.events {
		events = append(events, e)
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Remove oldest n events
	toRemove := n
	if toRemove > len(events) {
		toRemove = len(events)
	}
	for i := 0; i < toRemove; i++ {
		delete(l.events, events[i].ID)
	}
}

// LogDeployment logs a deployment-related event
func (l *InMemoryLogger) LogDeployment(resourceID, actor, description string, action EventAction, success bool, metadata map[string]string) (*AuditEvent, error) {
	severity := SeverityInfo
	if !success {
		severity = SeverityError
	}

	event := &AuditEvent{
		Type:         EventTypeDeployment,
		Action:       action,
		Severity:     severity,
		ResourceID:   resourceID,
		ResourceType: "deployment",
		Actor:        actor,
		Description:  description,
		Metadata:     metadata,
		Success:      success,
	}

	if err := l.Log(event); err != nil {
		return nil, err
	}
	return event, nil
}

// LogConfiguration logs a configuration change
func (l *InMemoryLogger) LogConfiguration(resourceID, actor string, oldValue, newValue interface{}, metadata map[string]string) (*AuditEvent, error) {
	action := ActionUpdate
	if oldValue == nil {
		action = ActionCreate
	}

	event := &AuditEvent{
		Type:         EventTypeConfiguration,
		Action:       action,
		Severity:     SeverityInfo,
		ResourceID:   resourceID,
		ResourceType: "configuration",
		Actor:        actor,
		Description:  fmt.Sprintf("Configuration %s for %s", action, resourceID),
		OldValue:     oldValue,
		NewValue:     newValue,
		Metadata:     metadata,
		Success:      true,
	}

	if err := l.Log(event); err != nil {
		return nil, err
	}
	return event, nil
}

// LogManifest logs a manifest-related event
func (l *InMemoryLogger) LogManifest(resourceID, actor, description string, action EventAction, success bool, metadata map[string]string) (*AuditEvent, error) {
	severity := SeverityInfo
	if !success {
		severity = SeverityError
	}

	event := &AuditEvent{
		Type:         EventTypeManifest,
		Action:       action,
		Severity:     severity,
		ResourceID:   resourceID,
		ResourceType: "manifest",
		Actor:        actor,
		Description:  description,
		Metadata:     metadata,
		Success:      success,
	}

	if err := l.Log(event); err != nil {
		return nil, err
	}
	return event, nil
}

// LogRollback logs a rollback event
func (l *InMemoryLogger) LogRollback(resourceID, actor string, fromVersion, toVersion string, success bool, metadata map[string]string) (*AuditEvent, error) {
	severity := SeverityWarning
	if !success {
		severity = SeverityError
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["from_version"] = fromVersion
	metadata["to_version"] = toVersion

	event := &AuditEvent{
		Type:         EventTypeRollback,
		Action:       ActionRollback,
		Severity:     severity,
		ResourceID:   resourceID,
		ResourceType: "deployment",
		Actor:        actor,
		Description:  fmt.Sprintf("Rollback from %s to %s", fromVersion, toVersion),
		Metadata:     metadata,
		Success:      success,
	}

	if err := l.Log(event); err != nil {
		return nil, err
	}
	return event, nil
}

// LogError logs an error event
func (l *InMemoryLogger) LogError(resourceID, actor, errorMessage string, metadata map[string]string) (*AuditEvent, error) {
	event := &AuditEvent{
		Type:         EventTypeError,
		Action:       ActionValidate,
		Severity:     SeverityError,
		ResourceID:   resourceID,
		ResourceType: "error",
		Actor:        actor,
		Description:  "Error occurred",
		Metadata:     metadata,
		Success:      false,
		ErrorMessage: errorMessage,
	}

	if err := l.Log(event); err != nil {
		return nil, err
	}
	return event, nil
}

// Get retrieves an event by ID
func (l *InMemoryLogger) Get(id string) (*AuditEvent, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	event, exists := l.events[id]
	return event, exists
}

// List returns all events sorted by timestamp (newest first)
func (l *InMemoryLogger) List() []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]AuditEvent, 0, len(l.events))
	for _, e := range l.events {
		result = append(result, *e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// Query filters events based on criteria
func (l *InMemoryLogger) Query(filter *AuditFilter) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if filter == nil {
		return l.List()
	}

	var result []AuditEvent
	for _, e := range l.events {
		if l.matchesFilter(e, filter) {
			result = append(result, *e)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []AuditEvent{}
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result
}

// matchesFilter checks if an event matches the filter criteria
func (l *InMemoryLogger) matchesFilter(event *AuditEvent, filter *AuditFilter) bool {
	// Type filter
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Action filter
	if len(filter.Actions) > 0 {
		found := false
		for _, a := range filter.Actions {
			if event.Action == a {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Severity filter
	if len(filter.Severities) > 0 {
		found := false
		for _, s := range filter.Severities {
			if event.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Resource ID filter
	if filter.ResourceID != "" && event.ResourceID != filter.ResourceID {
		return false
	}

	// Resource type filter
	if filter.ResourceType != "" && event.ResourceType != filter.ResourceType {
		return false
	}

	// Actor filter
	if filter.Actor != "" && event.Actor != filter.Actor {
		return false
	}

	// Correlation ID filter
	if filter.CorrelationID != "" && event.CorrelationID != filter.CorrelationID {
		return false
	}

	// Time filters
	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}

	// Success/failure filters
	if filter.SuccessOnly && !event.Success {
		return false
	}
	if filter.FailedOnly && event.Success {
		return false
	}

	return true
}

// GetByCorrelation returns events with the same correlation ID
func (l *InMemoryLogger) GetByCorrelation(correlationID string) []AuditEvent {
	return l.Query(&AuditFilter{CorrelationID: correlationID})
}

// GetSummary returns statistics about audit events
func (l *InMemoryLogger) GetSummary() *AuditSummary {
	l.mu.RLock()
	defer l.mu.RUnlock()

	summary := &AuditSummary{
		EventsByType:     make(map[EventType]int),
		EventsByAction:   make(map[EventAction]int),
		EventsBySeverity: make(map[EventSeverity]int),
	}

	actorCounts := make(map[string]int)
	resourceCounts := make(map[string]int)
	resourceTypes := make(map[string]string)
	var totalDuration time.Duration
	var durationCount int

	for _, e := range l.events {
		summary.TotalEvents++
		summary.EventsByType[e.Type]++
		summary.EventsByAction[e.Action]++
		summary.EventsBySeverity[e.Severity]++

		if e.Success {
			summary.SuccessCount++
		} else {
			summary.FailureCount++
		}

		if e.Actor != "" {
			actorCounts[e.Actor]++
		}
		if e.ResourceID != "" {
			resourceCounts[e.ResourceID]++
			resourceTypes[e.ResourceID] = e.ResourceType
		}

		if e.Duration > 0 {
			totalDuration += e.Duration
			durationCount++
		}

		if summary.FirstEvent == nil || e.Timestamp.Before(*summary.FirstEvent) {
			t := e.Timestamp
			summary.FirstEvent = &t
		}
		if summary.LastEvent == nil || e.Timestamp.After(*summary.LastEvent) {
			t := e.Timestamp
			summary.LastEvent = &t
		}
	}

	if durationCount > 0 {
		summary.AverageDuration = totalDuration / time.Duration(durationCount)
	}

	// Top actors
	for actor, count := range actorCounts {
		summary.TopActors = append(summary.TopActors, ActorStat{
			Actor:      actor,
			EventCount: count,
		})
	}
	sort.Slice(summary.TopActors, func(i, j int) bool {
		return summary.TopActors[i].EventCount > summary.TopActors[j].EventCount
	})
	if len(summary.TopActors) > 10 {
		summary.TopActors = summary.TopActors[:10]
	}

	// Top resources
	for resourceID, count := range resourceCounts {
		summary.TopResources = append(summary.TopResources, ResourceStat{
			ResourceID:   resourceID,
			ResourceType: resourceTypes[resourceID],
			EventCount:   count,
		})
	}
	sort.Slice(summary.TopResources, func(i, j int) bool {
		return summary.TopResources[i].EventCount > summary.TopResources[j].EventCount
	})
	if len(summary.TopResources) > 10 {
		summary.TopResources = summary.TopResources[:10]
	}

	return summary
}

// Export exports audit logs
func (l *InMemoryLogger) Export() (*AuditExport, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	events := make([]AuditEvent, 0, len(l.events))
	for _, e := range l.events {
		events = append(events, *e)
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return &AuditExport{
		Version:    "1.0",
		ExportedAt: time.Now().UTC(),
		Events:     events,
		Summary:    l.GetSummary(),
	}, nil
}

// Import imports audit logs
func (l *InMemoryLogger) Import(export *AuditExport) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if export == nil {
		return fmt.Errorf("export cannot be nil")
	}

	l.events = make(map[string]*AuditEvent)
	for i := range export.Events {
		e := export.Events[i]
		l.events[e.ID] = &e
	}

	return nil
}

// ToJSON serializes to JSON
func (l *InMemoryLogger) ToJSON() ([]byte, error) {
	export, err := l.Export()
	if err != nil {
		return nil, err
	}
	return json.Marshal(export)
}

// FromJSON deserializes from JSON
func (l *InMemoryLogger) FromJSON(data []byte) error {
	var export AuditExport
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("failed to unmarshal audit log: %w", err)
	}
	return l.Import(&export)
}

// Clear removes all events
func (l *InMemoryLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = make(map[string]*AuditEvent)
}

// Prune removes events older than the given time
func (l *InMemoryLogger) Prune(before time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	count := 0
	for id, e := range l.events {
		if e.Timestamp.Before(before) {
			delete(l.events, id)
			count++
		}
	}
	return count
}
