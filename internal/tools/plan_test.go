package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Test PlanManager
func TestPlanManager_SetAndGet(t *testing.T) {
	manager := NewPlanManager()

	plan := &Plan{
		TaskName: "Test task",
		Status:   "in_progress",
		Steps: []Step{
			{Description: "Step 1", Status: "active"},
		},
	}

	manager.SetPlan(plan)
	retrieved := manager.GetActivePlan()

	if retrieved == nil {
		t.Fatal("expected plan to be set, got nil")
	}
	if retrieved.TaskName != "Test task" {
		t.Errorf("expected TaskName 'Test task', got %s", retrieved.TaskName)
	}
}

func TestPlanManager_GetActiveStepDescription(t *testing.T) {
	manager := NewPlanManager()

	plan := &Plan{
		TaskName: "Test task",
		Status:   "in_progress",
		Steps: []Step{
			{Description: "Step 1", Status: "complete"},
			{Description: "Step 2", Status: "active"},
			{Description: "Step 3", Status: "pending"},
		},
	}
	manager.SetPlan(plan)

	desc := manager.GetActiveStepDescription()
	if desc != "Step 2" {
		t.Errorf("expected 'Step 2', got %s", desc)
	}
}

func TestPlanManager_IsComplete(t *testing.T) {
	manager := NewPlanManager()

	// Test with incomplete plan
	plan := &Plan{
		TaskName: "Test task",
		Status:   "in_progress",
		Steps: []Step{
			{Description: "Step 1", Status: "complete"},
			{Description: "Step 2", Status: "active"},
		},
	}
	manager.SetPlan(plan)

	if manager.IsComplete() {
		t.Error("expected plan to be incomplete")
	}

	// Test with complete plan
	plan.Steps[1].Status = "complete"
	manager.SetPlan(plan)

	if !manager.IsComplete() {
		t.Error("expected plan to be complete")
	}
}

func TestPlanManager_FormatActivePlan(t *testing.T) {
	manager := NewPlanManager()

	plan := &Plan{
		TaskName: "Test Authentication",
		Status:   "in_progress",
		Steps: []Step{
			{Description: "Create user model", Status: "complete"},
			{Description: "Create auth endpoints", Status: "active"},
			{Description: "Add validation", Status: "pending"},
		},
	}
	manager.SetPlan(plan)

	formatted := manager.FormatActivePlan()

	if !strings.Contains(formatted, "<active_plan>") {
		t.Error("expected <active_plan> tag")
	}
	if !strings.Contains(formatted, "Test Authentication") {
		t.Error("expected task name in output")
	}
	if !strings.Contains(formatted, "[✓]") {
		t.Error("expected checkmark for completed step")
	}
	if !strings.Contains(formatted, "[→]") {
		t.Error("expected arrow for active step")
	}
	if !strings.Contains(formatted, "[ ]") {
		t.Error("expected empty box for pending step")
	}
}

// Test PlanCreateTool
func TestPlanCreateTool_Success(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	if tool.Name() != "Plan.create" {
		t.Errorf("expected name 'Plan.create', got %s", tool.Name())
	}

	args := json.RawMessage(`{
		"task_name": "Add user authentication",
		"steps": [
			"Create user model",
			"Create auth endpoints",
			"Add validation"
		]
	}`)

	err := tool.Check(context.Background(), args)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	if resultMap["status"] != "created" {
		t.Errorf("expected status 'created', got %v", resultMap["status"])
	}

	if resultMap["active_step"] != "Create user model" {
		t.Errorf("expected active_step 'Create user model', got %v", resultMap["active_step"])
	}

	// Verify plan was stored
	plan := manager.GetActivePlan()
	if plan == nil {
		t.Fatal("expected plan to be stored")
	}
	if len(plan.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Status != "active" {
		t.Errorf("expected first step to be active, got %s", plan.Steps[0].Status)
	}
}

func TestPlanCreateTool_TooFewSteps(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	args := json.RawMessage(`{
		"task_name": "Simple task",
		"steps": ["Step 1", "Step 2"]
	}`)

	err := tool.Check(context.Background(), args)
	if err == nil {
		t.Error("expected error for plan with less than 3 steps")
	}
	if !strings.Contains(err.Error(), "at least 3 steps") {
		t.Errorf("expected 'at least 3 steps' error, got: %v", err)
	}
}

func TestPlanCreateTool_EmptyStepDescription(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	args := json.RawMessage(`{
		"task_name": "Test task",
		"steps": ["Step 1", "", "Step 3"]
	}`)

	err := tool.Check(context.Background(), args)
	if err == nil {
		t.Error("expected error for empty step description")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected 'cannot be empty' error, got: %v", err)
	}
}

func TestPlanCreateTool_RejectExistingPlan(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	// Create first plan
	args1 := json.RawMessage(`{
		"task_name": "Task 1",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = tool.Call(context.Background(), args1)

	// Try to create second plan without abandon flag
	args2 := json.RawMessage(`{
		"task_name": "Task 2",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)

	err := tool.Check(context.Background(), args2)
	if err == nil {
		t.Error("expected error when creating plan with existing active plan")
	}
	if !strings.Contains(err.Error(), "plan already exists") {
		t.Errorf("expected 'plan already exists' error, got: %v", err)
	}
}

func TestPlanCreateTool_AbandonPrevious(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	// Create first plan
	args1 := json.RawMessage(`{
		"task_name": "Task 1",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = tool.Call(context.Background(), args1)

	// Create second plan with abandon flag
	args2 := json.RawMessage(`{
		"task_name": "Task 2",
		"steps": ["New Step 1", "New Step 2", "New Step 3"],
		"abandon": true
	}`)

	err := tool.Check(context.Background(), args2)
	if err != nil {
		t.Fatalf("expected no error with abandon flag, got %v", err)
	}

	result, err := tool.Call(context.Background(), args2)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	plan := resultMap["plan"].(*Plan)
	if plan.TaskName != "Task 2" {
		t.Error("expected new task name in result")
	}
}

// Test PlanAddStepTool
func TestPlanAddStepTool_ToEnd(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	addTool := NewPlanAddStepTool(manager)

	if addTool.Name() != "Plan.addStep" {
		t.Errorf("expected name 'Plan.addStep', got %s", addTool.Name())
	}

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Add a new step to end (using at_end=true)
	addArgs := json.RawMessage(`{
		"description": "Step 4",
		"at_end": true
	}`)

	err := addTool.Check(context.Background(), addArgs)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	result, err := addTool.Call(context.Background(), addArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["status"] != "step_added" {
		t.Errorf("expected status 'step_added', got %v", resultMap["status"])
	}

	// Verify step was added at end
	plan := manager.GetActivePlan()
	if len(plan.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[3].Description != "Step 4" {
		t.Errorf("expected last step to be 'Step 4', got %s", plan.Steps[3].Description)
	}
}

func TestPlanAddStepTool_AfterActive(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	addTool := NewPlanAddStepTool(manager)

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Add a step after active (default behavior, no flag needed)
	addArgs := json.RawMessage(`{
		"description": "Inserted Step"
	}`)

	result, err := addTool.Call(context.Background(), addArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if !strings.Contains(resultMap["message"].(string), "Inserted") {
		t.Error("expected message to mention insertion")
	}

	// Verify step was inserted after active (step 1)
	plan := manager.GetActivePlan()
	if len(plan.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[1].Description != "Inserted Step" {
		t.Errorf("expected second step to be 'Inserted Step', got %s", plan.Steps[1].Description)
	}
}

func TestPlanAddStepTool_NoPlan(t *testing.T) {
	manager := NewPlanManager()
	addTool := NewPlanAddStepTool(manager)

	addArgs := json.RawMessage(`{
		"description": "New step"
	}`)

	err := addTool.Check(context.Background(), addArgs)
	if err == nil {
		t.Error("expected error when no active plan")
	}
	if !strings.Contains(err.Error(), "No active plan") {
		t.Errorf("expected 'No active plan' error, got: %v", err)
	}
}

func TestPlanAddStepTool_EmptyDescription(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	addTool := NewPlanAddStepTool(manager)

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	addArgs := json.RawMessage(`{
		"description": ""
	}`)

	err := addTool.Check(context.Background(), addArgs)
	if err == nil {
		t.Error("expected error for empty description")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected 'cannot be empty' error, got: %v", err)
	}
}

// Test PlanCompleteStepTool
func TestPlanCompleteStepTool_Success(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)

	if completeTool.Name() != "Plan.completeStep" {
		t.Errorf("expected name 'Plan.completeStep', got %s", completeTool.Name())
	}

	// Create plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Complete first step
	completeArgs := json.RawMessage(`{}`)

	err := completeTool.Check(context.Background(), completeArgs)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	result, err := completeTool.Call(context.Background(), completeArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["status"] != "step_completed" {
		t.Errorf("expected status 'step_completed', got %v", resultMap["status"])
	}
	if resultMap["completed_step"] != "Step 1" {
		t.Errorf("expected completed_step 'Step 1', got %v", resultMap["completed_step"])
	}
	if resultMap["active_step"] != "Step 2" {
		t.Errorf("expected active_step 'Step 2', got %v", resultMap["active_step"])
	}

	// Verify plan state
	plan := manager.GetActivePlan()
	if plan.Steps[0].Status != "complete" {
		t.Errorf("expected first step to be complete, got %s", plan.Steps[0].Status)
	}
	if plan.Steps[1].Status != "active" {
		t.Errorf("expected second step to be active, got %s", plan.Steps[1].Status)
	}
}

func TestPlanCompleteStepTool_AutoAdvances(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)

	// Create plan with three steps
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	completeArgs := json.RawMessage(`{}`)

	// Complete step 1
	_, _ = completeTool.Call(context.Background(), completeArgs)

	plan := manager.GetActivePlan()
	if plan.Steps[0].Status != "complete" {
		t.Errorf("step 1 should be complete, got %s", plan.Steps[0].Status)
	}
	if plan.Steps[1].Status != "active" {
		t.Errorf("step 2 should be active, got %s", plan.Steps[1].Status)
	}
	if plan.Steps[2].Status != "pending" {
		t.Errorf("step 3 should be pending, got %s", plan.Steps[2].Status)
	}

	// Complete step 2
	_, _ = completeTool.Call(context.Background(), completeArgs)

	plan = manager.GetActivePlan()
	if plan.Steps[1].Status != "complete" {
		t.Errorf("step 2 should be complete, got %s", plan.Steps[1].Status)
	}
	if plan.Steps[2].Status != "active" {
		t.Errorf("step 3 should be active, got %s", plan.Steps[2].Status)
	}
}

func TestPlanCompleteStepTool_MarksPlanComplete(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)

	// Create plan with three steps
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	completeArgs := json.RawMessage(`{}`)

	// Complete all steps
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 1
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 2
	result, _ := completeTool.Call(context.Background(), completeArgs) // Step 3

	resultMap := result.(map[string]any)
	if resultMap["status"] != "plan_complete" {
		t.Errorf("expected status 'plan_complete', got %v", resultMap["status"])
	}
	if resultMap["active_step"] != nil {
		t.Errorf("expected active_step to be nil, got %v", resultMap["active_step"])
	}

	// Verify plan status
	plan := manager.GetActivePlan()
	if plan.Status != "complete" {
		t.Errorf("expected plan status to be 'complete', got %s", plan.Status)
	}
}

func TestPlanCompleteStepTool_NoPlan(t *testing.T) {
	manager := NewPlanManager()
	completeTool := NewPlanCompleteStepTool(manager)

	completeArgs := json.RawMessage(`{}`)

	err := completeTool.Check(context.Background(), completeArgs)
	if err == nil {
		t.Error("expected error when no active plan")
	}
	if !strings.Contains(err.Error(), "No active plan") {
		t.Errorf("expected 'No active plan' error, got: %v", err)
	}
}

func TestPlanCompleteStepTool_PlanAlreadyComplete(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)

	// Create and complete a plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	completeArgs := json.RawMessage(`{}`)
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 1
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 2
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 3

	// Try to complete again
	err := completeTool.Check(context.Background(), completeArgs)
	if err == nil {
		t.Error("expected error when plan already complete")
	}
	if !strings.Contains(err.Error(), "already complete") {
		t.Errorf("expected 'already complete' error, got: %v", err)
	}
}

// Test PlanRemoveStepTool
func TestPlanRemoveStepTool_Success(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	removeTool := NewPlanRemoveStepTool(manager)

	if removeTool.Name() != "Plan.removeStep" {
		t.Errorf("expected name 'Plan.removeStep', got %s", removeTool.Name())
	}

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3", "Step 4"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Remove step 3 (pending)
	removeArgs := json.RawMessage(`{"step_number": 3}`)

	err := removeTool.Check(context.Background(), removeArgs)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	result, err := removeTool.Call(context.Background(), removeArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["status"] != "step_removed" {
		t.Errorf("expected status 'step_removed', got %v", resultMap["status"])
	}
	if resultMap["removed_step"] != "Step 3" {
		t.Errorf("expected removed_step 'Step 3', got %v", resultMap["removed_step"])
	}

	// Verify step was removed
	plan := manager.GetActivePlan()
	if len(plan.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan.Steps))
	}
	// Step 4 should now be at index 2
	if plan.Steps[2].Description != "Step 4" {
		t.Errorf("expected third step to be 'Step 4', got %s", plan.Steps[2].Description)
	}
}

func TestPlanRemoveStepTool_RemoveActiveAutoAdvances(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	removeTool := NewPlanRemoveStepTool(manager)

	// Create initial plan (step 1 is active)
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Remove step 1 (active) - should auto-advance to step 2
	removeArgs := json.RawMessage(`{"step_number": 1}`)

	err := removeTool.Check(context.Background(), removeArgs)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	result, err := removeTool.Call(context.Background(), removeArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["active_step"] != "Step 2" {
		t.Errorf("expected active_step 'Step 2', got %v", resultMap["active_step"])
	}

	// Verify plan state
	plan := manager.GetActivePlan()
	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Status != "active" {
		t.Errorf("expected first step to be active, got %s", plan.Steps[0].Status)
	}
	if plan.Steps[0].Description != "Step 2" {
		t.Errorf("expected first step to be 'Step 2', got %s", plan.Steps[0].Description)
	}
}

func TestPlanRemoveStepTool_CannotRemoveActiveWhenNoMorePending(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)
	removeTool := NewPlanRemoveStepTool(manager)

	// Create initial plan and complete first two steps
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)
	_, _ = completeTool.Call(context.Background(), json.RawMessage(`{}`)) // Complete step 1
	_, _ = completeTool.Call(context.Background(), json.RawMessage(`{}`)) // Complete step 2
	// Now step 3 is active and there are no pending steps

	// Try to remove step 3 (active with no pending steps)
	removeArgs := json.RawMessage(`{"step_number": 3}`)

	err := removeTool.Check(context.Background(), removeArgs)
	if err == nil {
		t.Error("expected error when removing active step with no pending steps")
	}
	if !strings.Contains(err.Error(), "no pending steps to activate") {
		t.Errorf("expected 'no pending steps to activate' error, got: %v", err)
	}
}

func TestPlanRemoveStepTool_CannotRemoveCompleted(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)
	removeTool := NewPlanRemoveStepTool(manager)

	// Create and complete step 1
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)
	_, _ = completeTool.Call(context.Background(), json.RawMessage(`{}`))

	// Try to remove step 1 (completed)
	removeArgs := json.RawMessage(`{"step_number": 1}`)

	err := removeTool.Check(context.Background(), removeArgs)
	if err == nil {
		t.Error("expected error when removing completed step")
	}
	if !strings.Contains(err.Error(), "Cannot remove a completed step") {
		t.Errorf("expected 'Cannot remove a completed step' error, got: %v", err)
	}
}

func TestPlanRemoveStepTool_InvalidStepNumber(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	removeTool := NewPlanRemoveStepTool(manager)

	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Try to remove step 5 (doesn't exist)
	removeArgs := json.RawMessage(`{"step_number": 5}`)

	err := removeTool.Check(context.Background(), removeArgs)
	if err == nil {
		t.Error("expected error for invalid step number")
	}
	if !strings.Contains(err.Error(), "Invalid step number") {
		t.Errorf("expected 'Invalid step number' error, got: %v", err)
	}
}

// Test PlanMoveStepTool
func TestPlanMoveStepTool_MoveForward(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	if moveTool.Name() != "Plan.moveStep" {
		t.Errorf("expected name 'Plan.moveStep', got %s", moveTool.Name())
	}

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3", "Step 4"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Move step 2 to position 4
	moveArgs := json.RawMessage(`{"from": 2, "to": 4}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	result, err := moveTool.Call(context.Background(), moveArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["status"] != "step_moved" {
		t.Errorf("expected status 'step_moved', got %v", resultMap["status"])
	}

	// Verify order: Step 1, Step 3, Step 4, Step 2
	plan := manager.GetActivePlan()
	expectedOrder := []string{"Step 1", "Step 3", "Step 4", "Step 2"}
	for i, expected := range expectedOrder {
		if plan.Steps[i].Description != expected {
			t.Errorf("expected step %d to be '%s', got '%s'", i+1, expected, plan.Steps[i].Description)
		}
	}
}

func TestPlanMoveStepTool_MoveBackward(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3", "Step 4"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Move step 4 to position 2
	moveArgs := json.RawMessage(`{"from": 4, "to": 2}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	_, _ = moveTool.Call(context.Background(), moveArgs)

	// Verify order: Step 1, Step 4, Step 2, Step 3
	plan := manager.GetActivePlan()
	expectedOrder := []string{"Step 1", "Step 4", "Step 2", "Step 3"}
	for i, expected := range expectedOrder {
		if plan.Steps[i].Description != expected {
			t.Errorf("expected step %d to be '%s', got '%s'", i+1, expected, plan.Steps[i].Description)
		}
	}
}

func TestPlanMoveStepTool_CannotMoveActive(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	// Create initial plan (step 1 is active)
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Try to move step 1 (active)
	moveArgs := json.RawMessage(`{"from": 1, "to": 3}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err == nil {
		t.Error("expected error when moving active step")
	}
	if !strings.Contains(err.Error(), "Cannot move the active step") {
		t.Errorf("expected 'Cannot move the active step' error, got: %v", err)
	}
}

func TestPlanMoveStepTool_CannotMoveCompleted(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	// Create and complete step 1
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)
	_, _ = completeTool.Call(context.Background(), json.RawMessage(`{}`))

	// Try to move step 1 (completed)
	moveArgs := json.RawMessage(`{"from": 1, "to": 3}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err == nil {
		t.Error("expected error when moving completed step")
	}
	if !strings.Contains(err.Error(), "Cannot move a completed step") {
		t.Errorf("expected 'Cannot move a completed step' error, got: %v", err)
	}
}

func TestPlanMoveStepTool_CannotMoveBeforeOrToActive(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	// Create and complete step 1, so step 2 becomes active
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3", "Step 4"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)
	_, _ = completeTool.Call(context.Background(), json.RawMessage(`{}`)) // Complete step 1, step 2 is now active

	// Try to move step 4 to position 1 (before active step at position 2)
	moveArgs := json.RawMessage(`{"from": 4, "to": 1}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err == nil {
		t.Error("expected error when moving before active step")
	}
	if !strings.Contains(err.Error(), "Cannot move step to or before the active step") {
		t.Errorf("expected 'Cannot move step to or before the active step' error, got: %v", err)
	}

	// Also try to move to position 2 (same as active step)
	moveArgs = json.RawMessage(`{"from": 4, "to": 2}`)

	err = moveTool.Check(context.Background(), moveArgs)
	if err == nil {
		t.Error("expected error when moving to active step position")
	}
	if !strings.Contains(err.Error(), "Cannot move step to or before the active step") {
		t.Errorf("expected 'Cannot move step to or before the active step' error, got: %v", err)
	}
}

func TestPlanMoveStepTool_CanMoveToAfterActive(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	// Create initial plan (step 1 is active at position 1)
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3", "Step 4"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Move step 4 to position 2 (right after active at position 1) - should be allowed
	moveArgs := json.RawMessage(`{"from": 4, "to": 2}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err != nil {
		t.Fatalf("should allow moving to position after active, got: %v", err)
	}

	_, _ = moveTool.Call(context.Background(), moveArgs)

	// Verify order: Step 1(active), Step 4, Step 2, Step 3
	plan := manager.GetActivePlan()
	if plan.Steps[0].Description != "Step 1" {
		t.Errorf("expected first step to be 'Step 1', got '%s'", plan.Steps[0].Description)
	}
	if plan.Steps[1].Description != "Step 4" {
		t.Errorf("expected second step to be 'Step 4', got '%s'", plan.Steps[1].Description)
	}
	if plan.Steps[2].Description != "Step 2" {
		t.Errorf("expected third step to be 'Step 2', got '%s'", plan.Steps[2].Description)
	}
}

func TestPlanMoveStepTool_CannotMoveToActivePosition(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	// Create initial plan (step 1 is active at position 1)
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3", "Step 4"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Try to move step 4 to position 1 (same as active) - should NOT be allowed
	moveArgs := json.RawMessage(`{"from": 4, "to": 1}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err == nil {
		t.Error("expected error when moving to active position")
	}
	if !strings.Contains(err.Error(), "Cannot move step to or before the active step") {
		t.Errorf("expected 'Cannot move step to or before the active step' error, got: %v", err)
	}
}

func TestPlanMoveStepTool_SamePosition(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	moveTool := NewPlanMoveStepTool(manager)

	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Try to move step 2 to position 2
	moveArgs := json.RawMessage(`{"from": 2, "to": 2}`)

	err := moveTool.Check(context.Background(), moveArgs)
	if err == nil {
		t.Error("expected error when moving to same position")
	}
	if !strings.Contains(err.Error(), "already at position") {
		t.Errorf("expected 'already at position' error, got: %v", err)
	}
}

func TestPlanCreateTool_DuplicateSteps(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	args := json.RawMessage(`{
		"task_name": "Test task",
		"steps": ["Step 1", "Step 2", "Step 1"]
	}`)

	err := tool.Check(context.Background(), args)
	if err == nil {
		t.Error("expected error for duplicate steps in plan")
	}
	if !strings.Contains(err.Error(), "Duplicate step") {
		t.Errorf("expected 'Duplicate step' error, got: %v", err)
	}
}

func TestPlanCreateTool_DuplicateStepsCaseInsensitive(t *testing.T) {
	manager := NewPlanManager()
	tool := NewPlanCreateTool(manager)

	args := json.RawMessage(`{
		"task_name": "Test task",
		"steps": ["Step One", "Step Two", "STEP ONE"]
	}`)

	err := tool.Check(context.Background(), args)
	if err == nil {
		t.Error("expected error for case-insensitive duplicate steps")
	}
	if !strings.Contains(err.Error(), "Duplicate step") {
		t.Errorf("expected 'Duplicate step' error, got: %v", err)
	}
}

func TestPlanAddStepTool_DuplicateStep(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	addTool := NewPlanAddStepTool(manager)

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Try to add a duplicate step
	addArgs := json.RawMessage(`{
		"description": "Step 2"
	}`)

	err := addTool.Check(context.Background(), addArgs)
	if err == nil {
		t.Error("expected error for duplicate step")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestPlanAddStepTool_DuplicateStepCaseInsensitive(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	addTool := NewPlanAddStepTool(manager)

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Try to add a duplicate step with different case
	addArgs := json.RawMessage(`{
		"description": "STEP 2"
	}`)

	err := addTool.Check(context.Background(), addArgs)
	if err == nil {
		t.Error("expected error for case-insensitive duplicate step")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestPlanAddStepTool_AllowDuplicateOfCompletedStep(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)
	addTool := NewPlanAddStepTool(manager)

	// Create initial plan
	createArgs := json.RawMessage(`{
		"task_name": "Test",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	// Complete the first step
	_, _ = completeTool.Call(context.Background(), json.RawMessage(`{}`))

	// Should be able to add a step with the same name as a completed step
	addArgs := json.RawMessage(`{
		"description": "Step 1"
	}`)

	err := addTool.Check(context.Background(), addArgs)
	if err != nil {
		t.Errorf("should allow adding step matching completed step, got: %v", err)
	}
}

func TestPlanCreateTool_AllowAfterComplete(t *testing.T) {
	manager := NewPlanManager()
	createTool := NewPlanCreateTool(manager)
	completeTool := NewPlanCompleteStepTool(manager)

	// Create and complete a plan
	createArgs := json.RawMessage(`{
		"task_name": "Task 1",
		"steps": ["Step 1", "Step 2", "Step 3"]
	}`)
	_, _ = createTool.Call(context.Background(), createArgs)

	completeArgs := json.RawMessage(`{}`)
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 1
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 2
	_, _ = completeTool.Call(context.Background(), completeArgs) // Step 3

	// Create new plan without abandon flag (should succeed since previous is complete)
	newArgs := json.RawMessage(`{
		"task_name": "Task 2",
		"steps": ["New Step 1", "New Step 2", "New Step 3"]
	}`)

	err := createTool.Check(context.Background(), newArgs)
	if err != nil {
		t.Fatalf("expected no error after completing previous plan, got: %v", err)
	}

	result, err := createTool.Call(context.Background(), newArgs)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	resultMap := result.(map[string]any)
	plan := resultMap["plan"].(*Plan)
	if plan.TaskName != "Task 2" {
		t.Errorf("expected task name 'Task 2', got %s", plan.TaskName)
	}
}
