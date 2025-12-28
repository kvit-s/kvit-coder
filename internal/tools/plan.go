package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Plan represents the active execution plan
type Plan struct {
	TaskName string `json:"task"`
	Status   string `json:"status,omitempty"` // "in_progress" | "complete"
	Steps    []Step `json:"steps"`
}

// Step represents a single step in the plan
type Step struct {
	Description string `json:"description"`
	Status      string `json:"status"` // "pending" | "active" | "complete"
}

// PlanManager manages the active plan
type PlanManager struct {
	mu   sync.RWMutex
	plan *Plan
}

// NewPlanManager creates a new plan manager
func NewPlanManager() *PlanManager {
	return &PlanManager{}
}

// GetActivePlan returns the currently active plan (if any)
func (pm *PlanManager) GetActivePlan() *Plan {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.plan
}

// SetPlan sets the active plan
func (pm *PlanManager) SetPlan(plan *Plan) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.plan = plan
}

// ClearPlan removes the active plan
func (pm *PlanManager) ClearPlan() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.plan = nil
}

// findActiveIndex returns the index of the active step, or -1 if none
func (pm *PlanManager) findActiveIndex() int {
	if pm.plan == nil {
		return -1
	}
	for i, step := range pm.plan.Steps {
		if step.Status == "active" {
			return i
		}
	}
	return -1
}

// GetActiveStepDescription returns the description of the active step
func (pm *PlanManager) GetActiveStepDescription() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.plan == nil {
		return ""
	}

	for _, step := range pm.plan.Steps {
		if step.Status == "active" {
			return step.Description
		}
	}
	return ""
}

// IsComplete returns true if the plan is complete
func (pm *PlanManager) IsComplete() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.plan == nil {
		return false
	}

	for _, step := range pm.plan.Steps {
		if step.Status != "complete" {
			return false
		}
	}
	return true
}

// FormatActivePlan formats the plan in <active_plan> XML format
func (pm *PlanManager) FormatActivePlan() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.plan == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<active_plan>\n")
	sb.WriteString(fmt.Sprintf("Task: %s\n\n", pm.plan.TaskName))
	sb.WriteString("Steps:\n")

	for i, step := range pm.plan.Steps {
		symbol := "[ ]"
		if step.Status == "complete" {
			symbol = "[✓]"
		} else if step.Status == "active" {
			symbol = "[→]"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, symbol, step.Description))
	}

	sb.WriteString("</active_plan>")
	return sb.String()
}

// =============================================================================
// plan.create - Create a new plan
// =============================================================================

type PlanCreateTool struct {
	manager *PlanManager
}

func NewPlanCreateTool(manager *PlanManager) *PlanCreateTool {
	return &PlanCreateTool{manager: manager}
}

func (t *PlanCreateTool) Name() string {
	return "Plan.create"
}

func (t *PlanCreateTool) Description() string {
	return "Create a new execution plan for multi-step tasks. First step automatically becomes active. Use for tasks with 3+ distinct steps."
}

func (t *PlanCreateTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_name": map[string]any{
				"type":        "string",
				"description": "Short name describing the task",
			},
			"steps": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"minItems":    3,
				"description": "List of step descriptions (minimum 3)",
			},
			"abandon": map[string]any{
				"type":        "boolean",
				"description": "Set true to abandon existing incomplete plan and start fresh",
			},
		},
		"required": []string{"task_name", "steps"},
	}
}

func (t *PlanCreateTool) PromptCategory() string { return "plan" }
func (t *PlanCreateTool) PromptOrder() int        { return 10 }
func (t *PlanCreateTool) PromptSection() string {
	return `You have tools for managing execution plans: Plan.create, Plan.completeStep, Plan.addStep, Plan.removeStep, Plan.moveStep.

### When to Create a Plan

**ALWAYS invoke ` + "`Plan.create`" + ` BEFORE starting work if:**
1. Task has 3+ distinct steps
2. Task involves multiple files or components
3. You naturally think "I will do X, then Y, then Z"
4. User explicitly requests a plan

**CRITICAL: If you need a plan, invoke ` + "`Plan.create`" + ` IMMEDIATELY as your first tool invocation.**

### Plan.create

Creates a new execution plan. Step 1 automatically becomes active.

**Parameters:**
- task_name: Brief description of the overall plan's task
- steps: Array of step descriptions (minimum 3 steps)
- abandon: Set to true to replace an existing incomplete plan

**Example:**
` + "```json" + `
{"task_name": "Fix authentication bug", "steps": ["Read AuthenticationForm code", "Fix the maxlength attribute", "Run tests to verify fix"]}
` + "```" + `

### Plan.completeStep

Marks the currently active step as complete. The next pending step automatically becomes active.

**No parameters required** - it always completes the current active step.

### Plan.addStep

Adds a new step to the plan. By default, inserts after the active step.

**Parameters:**
- description: Description of the new step
- at_end: If true, add to end of plan; otherwise insert after active step (default)

### Plan.removeStep

Removes a pending step from the plan.

**Parameters:**
- step_number: The step number to remove (1-indexed)

### Plan.moveStep

Moves a pending step to a different position. Cannot move to or before the active step.

**Parameters:**
- from: The step number to move (1-indexed)
- to: The target position (1-indexed, must be after active step)

### Workflow Example

` + "```" + `
User: "Fix the maxlength bug in AuthenticationForm"

1. Plan.create {"task_name": "Fix maxlength bug", "steps": ["Read code", "Fix bug", "Run tests"]}
   → Step 1 is now active

2. Read {"path": "forms.py"}
   → Do the work for step 1

3. Plan.completeStep {}
   → Step 1 complete, step 2 now active

4. Edit {"path": "forms.py", "start_line": 10, "end_line": 12, "new_text": "..."}
   → Do the work for step 2

5. Plan.completeStep {}
   → Step 2 complete, step 3 now active

6. Shell {"command": "pytest"}
   → Do the work for step 3

7. Plan.completeStep {}
   → Step 3 complete, plan finished!
` + "```" + `

### Rules

1. **Create plan FIRST** - before any work
2. **One active step** - only one step is active at a time
3. **Work then complete** - do the work, then call Plan.completeStep
4. **Auto-advance** - completing a step automatically activates the next one`
}

type planCreateArgs struct {
	TaskName string   `json:"task_name"`
	Steps    []string `json:"steps"`
	Abandon  bool     `json:"abandon"`
}

func (t *PlanCreateTool) Check(ctx context.Context, args json.RawMessage) error {
	var params planCreateArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.TaskName) == "" {
		return fmt.Errorf("task_name cannot be empty")
	}

	if len(params.Steps) < 3 {
		return fmt.Errorf("Plan must have at least 3 steps. For simpler tasks, just do the work without a plan.")
	}

	for i, step := range params.Steps {
		if strings.TrimSpace(step) == "" {
			return fmt.Errorf("Step descriptions cannot be empty (step %d is empty).", i+1)
		}
	}

	// Check for duplicate steps within the new plan
	seen := make(map[string]int)
	for i, step := range params.Steps {
		normalized := strings.ToLower(strings.TrimSpace(step))
		if prevIdx, exists := seen[normalized]; exists {
			return fmt.Errorf("Duplicate step found: step %d and step %d have the same description.", prevIdx+1, i+1)
		}
		seen[normalized] = i
	}

	// Check if there's an existing active plan
	activePlan := t.manager.GetActivePlan()
	if activePlan != nil && !t.manager.IsComplete() {
		if !params.Abandon {
			return fmt.Errorf("A plan already exists. Complete the current plan or use abandon=true to start fresh.")
		}
	}

	return nil
}

func (t *PlanCreateTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params planCreateArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Create new plan
	plan := &Plan{
		TaskName: strings.TrimSpace(params.TaskName),
		Status:   "in_progress",
		Steps:    make([]Step, 0, len(params.Steps)),
	}

	// Add steps - first step is active, rest are pending
	for i, stepDesc := range params.Steps {
		status := "pending"
		if i == 0 {
			status = "active"
		}
		plan.Steps = append(plan.Steps, Step{
			Description: strings.TrimSpace(stepDesc),
			Status:      status,
		})
	}

	// Store the plan
	t.manager.SetPlan(plan)

	return map[string]any{
		"status":      "created",
		"message":     fmt.Sprintf("Plan created with %d steps. Now active: '%s'", len(plan.Steps), plan.Steps[0].Description),
		"active_step": plan.Steps[0].Description,
		"plan":        plan,
	}, nil
}

// =============================================================================
// plan.add_step - Add a step to the plan
// =============================================================================

type PlanAddStepTool struct {
	manager *PlanManager
}

func NewPlanAddStepTool(manager *PlanManager) *PlanAddStepTool {
	return &PlanAddStepTool{manager: manager}
}

func (t *PlanAddStepTool) Name() string {
	return "Plan.addStep"
}

func (t *PlanAddStepTool) Description() string {
	return "Add a new step to the current plan. By default inserts after the current active step; use at_end=true to add to end of plan."
}

func (t *PlanAddStepTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Description of the new step",
			},
			"at_end": map[string]any{
				"type":        "boolean",
				"description": "Add to end of plan instead of after active step",
			},
		},
		"required": []string{"description"},
	}
}

func (t *PlanAddStepTool) PromptCategory() string { return "plan" }
func (t *PlanAddStepTool) PromptOrder() int        { return 30 }
func (t *PlanAddStepTool) PromptSection() string   { return "" } // Docs in plan.create

type planAddStepArgs struct {
	Description string `json:"description"`
	AtEnd       bool   `json:"at_end"`
}

func (t *PlanAddStepTool) Check(ctx context.Context, args json.RawMessage) error {
	var params planAddStepArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	plan := t.manager.GetActivePlan()
	if plan == nil {
		return fmt.Errorf("No active plan. Use plan.create to create a plan first.")
	}

	if strings.TrimSpace(params.Description) == "" {
		return fmt.Errorf("Step description cannot be empty.")
	}

	if t.manager.IsComplete() {
		return fmt.Errorf("Plan is already complete. Use plan.create to start a new plan.")
	}

	// Check for duplicate step - don't allow adding a step that matches an existing pending or active step
	newDesc := strings.TrimSpace(params.Description)
	for _, step := range plan.Steps {
		if step.Status != "complete" && strings.EqualFold(strings.TrimSpace(step.Description), newDesc) {
			return fmt.Errorf("Step '%s' already exists in the plan. Cannot add duplicate steps.", newDesc)
		}
	}

	return nil
}

func (t *PlanAddStepTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params planAddStepArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	plan := t.manager.GetActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("no active plan")
	}

	newStep := Step{
		Description: strings.TrimSpace(params.Description),
		Status:      "pending",
	}

	activeDesc := t.manager.GetActiveStepDescription()

	if params.AtEnd {
		// Add to end
		plan.Steps = append(plan.Steps, newStep)
		t.manager.SetPlan(plan)

		return map[string]any{
			"status":      "step_added",
			"message":     fmt.Sprintf("Added '%s' at end of plan. Still active: '%s'", newStep.Description, activeDesc),
			"active_step": activeDesc,
			"plan":        plan,
		}, nil
	}

	// Default: Insert after active step
	activeIdx := t.manager.findActiveIndex()
	if activeIdx >= 0 {
		plan.Steps = append(plan.Steps[:activeIdx+1], append([]Step{newStep}, plan.Steps[activeIdx+1:]...)...)
	} else {
		plan.Steps = append(plan.Steps, newStep)
	}

	t.manager.SetPlan(plan)
	return map[string]any{
		"status":      "step_added",
		"message":     fmt.Sprintf("Inserted '%s' after active step. Still active: '%s'", newStep.Description, activeDesc),
		"active_step": activeDesc,
		"plan":        plan,
	}, nil
}

// =============================================================================
// plan.remove_step - Remove a pending step from the plan
// =============================================================================

type PlanRemoveStepTool struct {
	manager *PlanManager
}

func NewPlanRemoveStepTool(manager *PlanManager) *PlanRemoveStepTool {
	return &PlanRemoveStepTool{manager: manager}
}

func (t *PlanRemoveStepTool) Name() string {
	return "Plan.removeStep"
}

func (t *PlanRemoveStepTool) Description() string {
	return "Remove a pending step from the plan by step number (1-indexed). Cannot remove active or completed steps."
}

func (t *PlanRemoveStepTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"step_number": map[string]any{
				"type":        "integer",
				"description": "The step number to remove (1-indexed)",
				"minimum":     1,
			},
		},
		"required": []string{"step_number"},
	}
}

func (t *PlanRemoveStepTool) PromptCategory() string { return "plan" }
func (t *PlanRemoveStepTool) PromptOrder() int        { return 40 }
func (t *PlanRemoveStepTool) PromptSection() string   { return "" } // Docs in plan.create

type planRemoveStepArgs struct {
	StepNumber int `json:"step_number"`
}

func (t *PlanRemoveStepTool) Check(ctx context.Context, args json.RawMessage) error {
	var params planRemoveStepArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	plan := t.manager.GetActivePlan()
	if plan == nil {
		return fmt.Errorf("No active plan. Use plan.create to create a plan first.")
	}

	if t.manager.IsComplete() {
		return fmt.Errorf("Plan is already complete. Use plan.create to start a new plan.")
	}

	if params.StepNumber < 1 || params.StepNumber > len(plan.Steps) {
		return fmt.Errorf("Invalid step number %d. Plan has %d steps.", params.StepNumber, len(plan.Steps))
	}

	step := plan.Steps[params.StepNumber-1]
	if step.Status == "complete" {
		return fmt.Errorf("Cannot remove a completed step.")
	}

	// Check if removing active step would leave no remaining steps
	if step.Status == "active" {
		hasPendingSteps := false
		for i, s := range plan.Steps {
			if i != params.StepNumber-1 && s.Status == "pending" {
				hasPendingSteps = true
				break
			}
		}
		if !hasPendingSteps {
			return fmt.Errorf("Cannot remove the active step when there are no pending steps to activate.")
		}
	}

	return nil
}

func (t *PlanRemoveStepTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params planRemoveStepArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	plan := t.manager.GetActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("no active plan")
	}

	idx := params.StepNumber - 1
	removedDesc := plan.Steps[idx].Description
	wasActive := plan.Steps[idx].Status == "active"

	// Remove the step
	plan.Steps = append(plan.Steps[:idx], plan.Steps[idx+1:]...)

	// If we removed the active step, activate the next pending step
	if wasActive {
		for i, s := range plan.Steps {
			if s.Status == "pending" {
				plan.Steps[i].Status = "active"
				break
			}
		}
	}

	t.manager.SetPlan(plan)

	activeDesc := t.manager.GetActiveStepDescription()

	return map[string]any{
		"status":       "step_removed",
		"message":      fmt.Sprintf("Removed step %d: '%s'. Now active: '%s'", params.StepNumber, removedDesc, activeDesc),
		"removed_step": removedDesc,
		"active_step":  activeDesc,
		"plan":         plan,
	}, nil
}

// =============================================================================
// plan.move_step - Move a pending step to a different position
// =============================================================================

type PlanMoveStepTool struct {
	manager *PlanManager
}

func NewPlanMoveStepTool(manager *PlanManager) *PlanMoveStepTool {
	return &PlanMoveStepTool{manager: manager}
}

func (t *PlanMoveStepTool) Name() string {
	return "Plan.moveStep"
}

func (t *PlanMoveStepTool) Description() string {
	return "Move a pending step to a different position by step number (1-indexed). Cannot move active or completed steps. Cannot move before completed steps."
}

func (t *PlanMoveStepTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"from": map[string]any{
				"type":        "integer",
				"description": "The step number to move (1-indexed)",
				"minimum":     1,
			},
			"to": map[string]any{
				"type":        "integer",
				"description": "The target position (1-indexed)",
				"minimum":     1,
			},
		},
		"required": []string{"from", "to"},
	}
}

func (t *PlanMoveStepTool) PromptCategory() string { return "plan" }
func (t *PlanMoveStepTool) PromptOrder() int        { return 50 }
func (t *PlanMoveStepTool) PromptSection() string   { return "" } // Docs in plan.create

type planMoveStepArgs struct {
	From int `json:"from"`
	To   int `json:"to"`
}

func (t *PlanMoveStepTool) Check(ctx context.Context, args json.RawMessage) error {
	var params planMoveStepArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	plan := t.manager.GetActivePlan()
	if plan == nil {
		return fmt.Errorf("No active plan. Use plan.create to create a plan first.")
	}

	if t.manager.IsComplete() {
		return fmt.Errorf("Plan is already complete. Use plan.create to start a new plan.")
	}

	if params.From < 1 || params.From > len(plan.Steps) {
		return fmt.Errorf("Invalid 'from' step number %d. Plan has %d steps.", params.From, len(plan.Steps))
	}

	if params.To < 1 || params.To > len(plan.Steps) {
		return fmt.Errorf("Invalid 'to' position %d. Plan has %d steps.", params.To, len(plan.Steps))
	}

	if params.From == params.To {
		return fmt.Errorf("Step is already at position %d.", params.From)
	}

	step := plan.Steps[params.From-1]
	if step.Status == "active" {
		return fmt.Errorf("Cannot move the active step.")
	}
	if step.Status == "complete" {
		return fmt.Errorf("Cannot move a completed step.")
	}

	// Find the position of the active step - cannot move before it
	activePos := 0
	for i, s := range plan.Steps {
		if s.Status == "active" {
			activePos = i + 1 // 1-indexed
			break
		}
	}

	if params.To <= activePos {
		return fmt.Errorf("Cannot move step to or before the active step. Earliest valid position is %d.", activePos+1)
	}

	return nil
}

func (t *PlanMoveStepTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params planMoveStepArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	plan := t.manager.GetActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("no active plan")
	}

	fromIdx := params.From - 1
	toIdx := params.To - 1

	// Extract the step to move
	step := plan.Steps[fromIdx]

	// Remove from original position
	plan.Steps = append(plan.Steps[:fromIdx], plan.Steps[fromIdx+1:]...)

	// Insert at target position
	plan.Steps = append(plan.Steps[:toIdx], append([]Step{step}, plan.Steps[toIdx:]...)...)

	t.manager.SetPlan(plan)

	activeDesc := t.manager.GetActiveStepDescription()

	return map[string]any{
		"status":      "step_moved",
		"message":     fmt.Sprintf("Moved '%s' from position %d to %d. Still active: '%s'", step.Description, params.From, params.To, activeDesc),
		"moved_step":  step.Description,
		"active_step": activeDesc,
		"plan":        plan,
	}, nil
}

// =============================================================================
// plan.complete_step - Mark the active step as complete
// =============================================================================

type PlanCompleteStepTool struct {
	manager *PlanManager
}

func NewPlanCompleteStepTool(manager *PlanManager) *PlanCompleteStepTool {
	return &PlanCompleteStepTool{manager: manager}
}

func (t *PlanCompleteStepTool) Name() string {
	return "Plan.completeStep"
}

func (t *PlanCompleteStepTool) Description() string {
	return "Mark the currently active step as complete. Next pending step automatically becomes active. Call AFTER doing the work, not before."
}

func (t *PlanCompleteStepTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *PlanCompleteStepTool) PromptCategory() string { return "plan" }
func (t *PlanCompleteStepTool) PromptOrder() int        { return 20 }
func (t *PlanCompleteStepTool) PromptSection() string   { return "" } // Docs in plan.create

func (t *PlanCompleteStepTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.manager.GetActivePlan() == nil {
		return fmt.Errorf("No active plan. Use plan.create to create a plan first.")
	}

	if t.manager.IsComplete() {
		return fmt.Errorf("Plan is already complete. Use plan.create to start a new plan.")
	}

	return nil
}

func (t *PlanCompleteStepTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	plan := t.manager.GetActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("no active plan")
	}

	// Find and complete the active step
	activeIdx := -1
	var completedDesc string
	for i, step := range plan.Steps {
		if step.Status == "active" {
			activeIdx = i
			completedDesc = step.Description
			plan.Steps[i].Status = "complete"
			break
		}
	}

	if activeIdx == -1 {
		return nil, fmt.Errorf("no active step to complete")
	}

	// Find and activate the next pending step
	nextActiveDesc := ""
	allComplete := true
	for i := activeIdx + 1; i < len(plan.Steps); i++ {
		if plan.Steps[i].Status == "pending" {
			plan.Steps[i].Status = "active"
			nextActiveDesc = plan.Steps[i].Description
			allComplete = false
			break
		}
	}

	// If no pending step found after, check if there are any pending steps at all
	if nextActiveDesc == "" {
		for i, step := range plan.Steps {
			if step.Status == "pending" {
				plan.Steps[i].Status = "active"
				nextActiveDesc = step.Description
				allComplete = false
				break
			}
		}
	}

	if allComplete {
		plan.Status = "complete"
	}

	t.manager.SetPlan(plan)

	if allComplete {
		return map[string]any{
			"status":         "plan_complete",
			"message":        fmt.Sprintf("Completed '%s'. All steps done - plan finished!", completedDesc),
			"completed_step": completedDesc,
			"active_step":    nil,
			"plan":           plan,
		}, nil
	}

	return map[string]any{
		"status":         "step_completed",
		"message":        fmt.Sprintf("Completed '%s'. Now active: '%s'", completedDesc, nextActiveDesc),
		"completed_step": completedDesc,
		"active_step":    nextActiveDesc,
		"plan":           plan,
	}, nil
}
