package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryLogger(t *testing.T) {
	logger := NewInMemoryLogger(100)
	assert.NotNil(t, logger)
	assert.Equal(t, 100, logger.maxSize)
}

func TestNewInMemoryLogger_DefaultMaxSize(t *testing.T) {
	logger := NewInMemoryLogger(0)
	assert.Equal(t, 10000, logger.maxSize)

	logger = NewInMemoryLogger(-1)
	assert.Equal(t, 10000, logger.maxSize)
}

func TestInMemoryLogger_Log(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event := &AuditEvent{
		Type:         EventTypeDeployment,
		Action:       ActionCreate,
		Severity:     SeverityInfo,
		ResourceID:   "test-resource",
		ResourceType: "deployment",
		Actor:        "test-user",
		Description:  "Test deployment",
		Success:      true,
	}

	err := logger.Log(event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
	assert.False(t, event.Timestamp.IsZero())
}

func TestInMemoryLogger_Log_NilEvent(t *testing.T) {
	logger := NewInMemoryLogger(100)

	err := logger.Log(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestInMemoryLogger_Log_PreservesExistingID(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event := &AuditEvent{
		ID:           "custom-id",
		Type:         EventTypeDeployment,
		Action:       ActionCreate,
		ResourceID:   "test-resource",
		ResourceType: "deployment",
		Success:      true,
	}

	err := logger.Log(event)
	require.NoError(t, err)
	assert.Equal(t, "custom-id", event.ID)
}

func TestInMemoryLogger_Log_MaxSizePruning(t *testing.T) {
	logger := NewInMemoryLogger(10)

	for i := 0; i < 15; i++ {
		event := &AuditEvent{
			Type:       EventTypeDeployment,
			Action:     ActionCreate,
			ResourceID: "test-resource",
			Success:    true,
		}
		err := logger.Log(event)
		require.NoError(t, err)
	}

	// Should have pruned some events
	events := logger.List()
	assert.True(t, len(events) <= 10)
}

func TestInMemoryLogger_LogDeployment(t *testing.T) {
	logger := NewInMemoryLogger(100)

	metadata := map[string]string{"env": "production"}
	event, err := logger.LogDeployment("deploy-1", "admin", "Deployed app", ActionApply, true, metadata)

	require.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, EventTypeDeployment, event.Type)
	assert.Equal(t, ActionApply, event.Action)
	assert.Equal(t, SeverityInfo, event.Severity)
	assert.Equal(t, "deploy-1", event.ResourceID)
	assert.Equal(t, "admin", event.Actor)
	assert.True(t, event.Success)
}

func TestInMemoryLogger_LogDeployment_Failed(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event, err := logger.LogDeployment("deploy-1", "admin", "Failed deploy", ActionApply, false, nil)

	require.NoError(t, err)
	assert.Equal(t, SeverityError, event.Severity)
	assert.False(t, event.Success)
}

func TestInMemoryLogger_LogConfiguration(t *testing.T) {
	logger := NewInMemoryLogger(100)

	oldValue := map[string]string{"setting": "old"}
	newValue := map[string]string{"setting": "new"}

	event, err := logger.LogConfiguration("config-1", "admin", oldValue, newValue, nil)

	require.NoError(t, err)
	assert.Equal(t, EventTypeConfiguration, event.Type)
	assert.Equal(t, ActionUpdate, event.Action)
	assert.Equal(t, oldValue, event.OldValue)
	assert.Equal(t, newValue, event.NewValue)
}

func TestInMemoryLogger_LogConfiguration_Create(t *testing.T) {
	logger := NewInMemoryLogger(100)

	newValue := map[string]string{"setting": "new"}

	event, err := logger.LogConfiguration("config-1", "admin", nil, newValue, nil)

	require.NoError(t, err)
	assert.Equal(t, ActionCreate, event.Action)
}

func TestInMemoryLogger_LogManifest(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event, err := logger.LogManifest("manifest-1", "system", "Applied manifest", ActionApply, true, nil)

	require.NoError(t, err)
	assert.Equal(t, EventTypeManifest, event.Type)
	assert.Equal(t, "manifest", event.ResourceType)
}

func TestInMemoryLogger_LogRollback(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event, err := logger.LogRollback("deploy-1", "admin", "v2", "v1", true, nil)

	require.NoError(t, err)
	assert.Equal(t, EventTypeRollback, event.Type)
	assert.Equal(t, ActionRollback, event.Action)
	assert.Equal(t, SeverityWarning, event.Severity)
	assert.Equal(t, "v2", event.Metadata["from_version"])
	assert.Equal(t, "v1", event.Metadata["to_version"])
}

func TestInMemoryLogger_LogRollback_Failed(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event, err := logger.LogRollback("deploy-1", "admin", "v2", "v1", false, nil)

	require.NoError(t, err)
	assert.Equal(t, SeverityError, event.Severity)
}

func TestInMemoryLogger_LogError(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event, err := logger.LogError("resource-1", "system", "Something went wrong", nil)

	require.NoError(t, err)
	assert.Equal(t, EventTypeError, event.Type)
	assert.Equal(t, SeverityError, event.Severity)
	assert.False(t, event.Success)
	assert.Equal(t, "Something went wrong", event.ErrorMessage)
}

func TestInMemoryLogger_Get(t *testing.T) {
	logger := NewInMemoryLogger(100)

	event := &AuditEvent{
		ID:         "test-id",
		Type:       EventTypeDeployment,
		Action:     ActionCreate,
		ResourceID: "test",
		Success:    true,
	}
	_ = logger.Log(event)

	retrieved, exists := logger.Get("test-id")
	assert.True(t, exists)
	assert.Equal(t, "test-id", retrieved.ID)

	_, exists = logger.Get("nonexistent")
	assert.False(t, exists)
}

func TestInMemoryLogger_List(t *testing.T) {
	logger := NewInMemoryLogger(100)

	// Log events with different timestamps
	now := time.Now()
	events := []*AuditEvent{
		{ID: "1", Timestamp: now.Add(-2 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{ID: "2", Timestamp: now.Add(-1 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{ID: "3", Timestamp: now, Type: EventTypeDeployment, Action: ActionCreate, Success: true},
	}

	for _, e := range events {
		_ = logger.Log(e)
	}

	list := logger.List()
	assert.Len(t, list, 3)
	// Should be sorted newest first
	assert.Equal(t, "3", list[0].ID)
	assert.Equal(t, "2", list[1].ID)
	assert.Equal(t, "1", list[2].ID)
}

func TestInMemoryLogger_Query_ByType(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Deploy", ActionApply, true, nil)
	_, _ = logger.LogManifest("m1", "user", "Manifest", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "Deploy 2", ActionApply, true, nil)

	results := logger.Query(&AuditFilter{Types: []EventType{EventTypeDeployment}})
	assert.Len(t, results, 2)
}

func TestInMemoryLogger_Query_ByAction(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Create", ActionCreate, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "Update", ActionUpdate, true, nil)
	_, _ = logger.LogDeployment("d3", "user", "Delete", ActionDelete, true, nil)

	results := logger.Query(&AuditFilter{Actions: []EventAction{ActionCreate, ActionDelete}})
	assert.Len(t, results, 2)
}

func TestInMemoryLogger_Query_BySeverity(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Success", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "Failed", ActionApply, false, nil)
	_, _ = logger.LogError("e1", "system", "Error", nil)

	results := logger.Query(&AuditFilter{Severities: []EventSeverity{SeverityError}})
	assert.Len(t, results, 2)
}

func TestInMemoryLogger_Query_ByResourceID(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Deploy 1", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "Deploy 2", ActionApply, true, nil)

	results := logger.Query(&AuditFilter{ResourceID: "d1"})
	assert.Len(t, results, 1)
	assert.Equal(t, "d1", results[0].ResourceID)
}

func TestInMemoryLogger_Query_ByActor(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "admin", "Admin deploy", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "User deploy", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d3", "admin", "Admin deploy 2", ActionApply, true, nil)

	results := logger.Query(&AuditFilter{Actor: "admin"})
	assert.Len(t, results, 2)
}

func TestInMemoryLogger_Query_ByTimeRange(t *testing.T) {
	logger := NewInMemoryLogger(100)
	now := time.Now()

	events := []*AuditEvent{
		{Timestamp: now.Add(-3 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{Timestamp: now.Add(-1 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{Timestamp: now.Add(1 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
	}
	for _, e := range events {
		_ = logger.Log(e)
	}

	startTime := now.Add(-2 * time.Hour)
	endTime := now
	results := logger.Query(&AuditFilter{StartTime: &startTime, EndTime: &endTime})
	assert.Len(t, results, 1)
}

func TestInMemoryLogger_Query_SuccessOnly(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Success", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "Failed", ActionApply, false, nil)

	results := logger.Query(&AuditFilter{SuccessOnly: true})
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
}

func TestInMemoryLogger_Query_FailedOnly(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Success", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "user", "Failed", ActionApply, false, nil)

	results := logger.Query(&AuditFilter{FailedOnly: true})
	assert.Len(t, results, 1)
	assert.False(t, results[0].Success)
}

func TestInMemoryLogger_Query_LimitOffset(t *testing.T) {
	logger := NewInMemoryLogger(100)

	for i := 0; i < 10; i++ {
		_, _ = logger.LogDeployment("d", "user", "Deploy", ActionApply, true, nil)
	}

	// Test limit
	results := logger.Query(&AuditFilter{Limit: 3})
	assert.Len(t, results, 3)

	// Test offset
	results = logger.Query(&AuditFilter{Offset: 5})
	assert.Len(t, results, 5)

	// Test limit and offset together
	results = logger.Query(&AuditFilter{Limit: 3, Offset: 2})
	assert.Len(t, results, 3)

	// Test offset beyond results
	results = logger.Query(&AuditFilter{Offset: 20})
	assert.Len(t, results, 0)
}

func TestInMemoryLogger_Query_NilFilter(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Deploy", ActionApply, true, nil)

	results := logger.Query(nil)
	assert.Len(t, results, 1)
}

func TestInMemoryLogger_GetByCorrelation(t *testing.T) {
	logger := NewInMemoryLogger(100)

	correlationID := "corr-123"
	events := []*AuditEvent{
		{CorrelationID: correlationID, Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{CorrelationID: correlationID, Type: EventTypeManifest, Action: ActionApply, Success: true},
		{CorrelationID: "other", Type: EventTypeDeployment, Action: ActionCreate, Success: true},
	}
	for _, e := range events {
		_ = logger.Log(e)
	}

	results := logger.GetByCorrelation(correlationID)
	assert.Len(t, results, 2)
}

func TestInMemoryLogger_GetSummary(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "admin", "Deploy 1", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "admin", "Deploy 2", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d3", "user", "Deploy 3", ActionApply, false, nil)
	_, _ = logger.LogManifest("m1", "system", "Manifest", ActionCreate, true, nil)

	summary := logger.GetSummary()

	assert.Equal(t, 4, summary.TotalEvents)
	assert.Equal(t, 3, summary.EventsByType[EventTypeDeployment])
	assert.Equal(t, 1, summary.EventsByType[EventTypeManifest])
	assert.Equal(t, 3, summary.SuccessCount)
	assert.Equal(t, 1, summary.FailureCount)
	assert.NotNil(t, summary.FirstEvent)
	assert.NotNil(t, summary.LastEvent)
	assert.True(t, len(summary.TopActors) > 0)
}

func TestInMemoryLogger_Export_Import(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "admin", "Deploy", ActionApply, true, nil)
	_, _ = logger.LogManifest("m1", "system", "Manifest", ActionCreate, true, nil)

	export, err := logger.Export()
	require.NoError(t, err)
	assert.NotEmpty(t, export.Version)
	assert.NotNil(t, export.Summary)
	assert.Len(t, export.Events, 2)

	newLogger := NewInMemoryLogger(100)
	err = newLogger.Import(export)
	require.NoError(t, err)

	events := newLogger.List()
	assert.Len(t, events, 2)
}

func TestInMemoryLogger_Import_Nil(t *testing.T) {
	logger := NewInMemoryLogger(100)

	err := logger.Import(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestInMemoryLogger_ToJSON_FromJSON(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "admin", "Deploy", ActionApply, true, nil)

	jsonData, err := logger.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Verify it's valid JSON
	var export AuditExport
	err = json.Unmarshal(jsonData, &export)
	require.NoError(t, err)

	newLogger := NewInMemoryLogger(100)
	err = newLogger.FromJSON(jsonData)
	require.NoError(t, err)

	events := newLogger.List()
	assert.Len(t, events, 1)
}

func TestInMemoryLogger_FromJSON_Invalid(t *testing.T) {
	logger := NewInMemoryLogger(100)

	err := logger.FromJSON([]byte("invalid json"))
	assert.Error(t, err)
}

func TestInMemoryLogger_Clear(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "admin", "Deploy", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "admin", "Deploy 2", ActionApply, true, nil)

	assert.Len(t, logger.List(), 2)

	logger.Clear()

	assert.Len(t, logger.List(), 0)
}

func TestInMemoryLogger_Prune(t *testing.T) {
	logger := NewInMemoryLogger(100)
	now := time.Now()

	events := []*AuditEvent{
		{Timestamp: now.Add(-3 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{Timestamp: now.Add(-2 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{Timestamp: now.Add(-1 * time.Hour), Type: EventTypeDeployment, Action: ActionCreate, Success: true},
		{Timestamp: now, Type: EventTypeDeployment, Action: ActionCreate, Success: true},
	}
	for _, e := range events {
		_ = logger.Log(e)
	}

	// Prune events older than 90 minutes ago
	pruned := logger.Prune(now.Add(-90 * time.Minute))
	assert.Equal(t, 2, pruned)
	assert.Len(t, logger.List(), 2)
}

func TestEventTypes(t *testing.T) {
	assert.Equal(t, EventType("deployment"), EventTypeDeployment)
	assert.Equal(t, EventType("configuration"), EventTypeConfiguration)
	assert.Equal(t, EventType("manifest"), EventTypeManifest)
	assert.Equal(t, EventType("rollback"), EventTypeRollback)
	assert.Equal(t, EventType("state"), EventTypeState)
	assert.Equal(t, EventType("error"), EventTypeError)
}

func TestEventActions(t *testing.T) {
	assert.Equal(t, EventAction("create"), ActionCreate)
	assert.Equal(t, EventAction("update"), ActionUpdate)
	assert.Equal(t, EventAction("delete"), ActionDelete)
	assert.Equal(t, EventAction("apply"), ActionApply)
	assert.Equal(t, EventAction("rollback"), ActionRollback)
	assert.Equal(t, EventAction("validate"), ActionValidate)
	assert.Equal(t, EventAction("migrate"), ActionMigrate)
}

func TestEventSeverities(t *testing.T) {
	assert.Equal(t, EventSeverity("info"), SeverityInfo)
	assert.Equal(t, EventSeverity("warning"), SeverityWarning)
	assert.Equal(t, EventSeverity("error"), SeverityError)
	assert.Equal(t, EventSeverity("critical"), SeverityCritical)
}

func TestAuditEvent_Timestamps(t *testing.T) {
	logger := NewInMemoryLogger(100)

	before := time.Now().Add(-time.Second)
	event, _ := logger.LogDeployment("d1", "admin", "Deploy", ActionApply, true, nil)
	after := time.Now().Add(time.Second)

	assert.True(t, event.Timestamp.After(before))
	assert.True(t, event.Timestamp.Before(after))
}

func TestInMemoryLogger_Query_ResourceType(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "user", "Deploy", ActionApply, true, nil)
	_, _ = logger.LogManifest("m1", "user", "Manifest", ActionApply, true, nil)

	results := logger.Query(&AuditFilter{ResourceType: "deployment"})
	assert.Len(t, results, 1)

	results = logger.Query(&AuditFilter{ResourceType: "manifest"})
	assert.Len(t, results, 1)
}

func TestInMemoryLogger_Query_Combined(t *testing.T) {
	logger := NewInMemoryLogger(100)

	_, _ = logger.LogDeployment("d1", "admin", "Deploy 1", ActionApply, true, nil)
	_, _ = logger.LogDeployment("d2", "admin", "Deploy 2", ActionApply, false, nil)
	_, _ = logger.LogDeployment("d3", "user", "Deploy 3", ActionApply, true, nil)
	_, _ = logger.LogManifest("m1", "admin", "Manifest", ActionApply, true, nil)

	// Combine type and actor
	results := logger.Query(&AuditFilter{
		Types: []EventType{EventTypeDeployment},
		Actor: "admin",
	})
	assert.Len(t, results, 2)

	// Combine type, actor, and success
	results = logger.Query(&AuditFilter{
		Types:       []EventType{EventTypeDeployment},
		Actor:       "admin",
		SuccessOnly: true,
	})
	assert.Len(t, results, 1)
}
