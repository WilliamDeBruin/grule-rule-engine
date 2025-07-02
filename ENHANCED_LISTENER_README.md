# Enhanced GruleEngineListener Interface

This PR enhances the `GruleEngineListener` interface to provide:

1. **Data Context Access**: Expose the `ast.IDataContext` to listeners for advanced inspection
2. **Pre/Post Evaluation and Execution Hooks**: Add granular timing hooks around rule evaluation and execution  
3. **Error Capture**: Access to evaluation and execution errors for advanced error handling
4. **Backward Compatibility**: Maintain compatibility with existing listeners

## Changes Made

### 1. New Enhanced Interface

Added `EnhancedGruleEngineListener` interface in `engine/GruleEngineListener.go`:

```go
type EnhancedGruleEngineListener interface {
    GruleEngineListener
    
    // PreEvaluateRuleEntry called before evaluating a rule's condition
    PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext)
    
    // PostEvaluateRuleEntry called after evaluating a rule's condition
    PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error)
    
    // PreExecuteRuleEntry called immediately before rule execution 
    PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext)
    
    // PostExecuteRuleEntry called immediately after rule execution
    PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error)
    
    // BeginCycleWithContext provides data context access at cycle start
    BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext)
}
```

### 2. Engine Modifications

Modified `engine/GruleEngine.go` to:

- Add separate pre/post notification methods for evaluation and execution phases
- Call enhanced hooks at the appropriate points in the rule processing lifecycle
- Pass `ast.IDataContext` to all enhanced methods for data inspection
- Capture and pass errors from evaluation and execution phases
- Maintain backward compatibility with existing listeners

### 3. Hook Execution Flow

The engine now calls hooks in this precise order:

1. **Cycle Start**: `BeginCycle()` and `BeginCycleWithContext()`
2. **Rule Evaluation**:
   - `PreEvaluateRuleEntry()` - before evaluating rule condition
   - `EvaluateRuleEntry()` - existing basic hook  
   - `PostEvaluateRuleEntry()` - after evaluating rule condition (with error info)
3. **Rule Execution** (if rule matched):
   - `PreExecuteRuleEntry()` - before executing rule actions
   - `ExecuteRuleEntry()` - existing basic hook
   - `PostExecuteRuleEntry()` - after executing rule actions (with error info)

### 4. Example Usage

#### Advanced Metrics Listener with Timing and Error Handling
```go
type MetricsListener struct {
    evaluationTimes map[string]time.Time
    executionTimes  map[string]time.Time
    evaluationErrors []error
    executionErrors  []error
}

func (m *MetricsListener) PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
    m.evaluationTimes[entry.RuleName] = time.Now()
    
    // Inspect data context before evaluation
    keys := dataCtx.GetKeys()
    log.Printf("Evaluating rule %s with data keys: %v", entry.RuleName, keys)
}

func (m *MetricsListener) PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error) {
    // Calculate evaluation time
    if startTime, exists := m.evaluationTimes[entry.RuleName]; exists {
        duration := time.Since(startTime)
        log.Printf("Rule %s evaluation took %v, matched: %v", entry.RuleName, duration, candidate)
        delete(m.evaluationTimes, entry.RuleName)
    }
    
    // Track evaluation errors
    if execError != nil {
        m.evaluationErrors = append(m.evaluationErrors, execError)
        log.Printf("Rule %s evaluation error: %v", entry.RuleName, execError)
    }
}

func (m *MetricsListener) PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
    m.executionTimes[entry.RuleName] = time.Now()
}

func (m *MetricsListener) PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error) {
    // Calculate execution time
    if startTime, exists := m.executionTimes[entry.RuleName]; exists {
        duration := time.Since(startTime)
        log.Printf("Rule %s execution took %v", entry.RuleName, duration)
        delete(m.executionTimes, entry.RuleName)
    }
    
    // Track execution errors
    if execError != nil {
        m.executionErrors = append(m.executionErrors, execError)
        log.Printf("Rule %s execution error: %v", entry.RuleName, execError)
    }
}

// Implement basic interface for compatibility
func (m *MetricsListener) EvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool) {}
func (m *MetricsListener) ExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry) {}
func (m *MetricsListener) BeginCycle(ctx context.Context, cycle uint64) {}
func (m *MetricsListener) BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext) {}
```

#### Shadow Mode Listener (Non-Intrusive Testing)
```go
type ShadowModeListener struct {
    shadowResults map[string]interface{}
}

func (s *ShadowModeListener) PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
    // Create shadow copy of data context for testing
    // Could implement shadow evaluation logic here
}

func (s *ShadowModeListener) PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error) {
    // Compare shadow results with actual results
    // Log differences for analysis
}

// Other methods...
```

## Benefits

1. **Precise Timing**: Separate pre/post hooks for evaluation and execution phases enable accurate performance measurement
2. **Data Context Inspection**: Full access to data context at all phases for advanced monitoring and debugging
3. **Error Handling**: Access to both evaluation and execution errors for comprehensive error tracking
4. **Shadow Mode Support**: Non-intrusive testing by inspecting data context without modification
5. **Backward Compatibility**: Existing listeners continue to work without any changes
6. **Flexible Implementation**: Choose which hooks to implement based on specific needs

## Testing

Comprehensive test suite validates:
- **Backward Compatibility**: `TestBasicListenerBackwardCompatibility` ensures existing listeners work unchanged
- **Pre/Post Evaluation Hooks**: `TestEnhancedListenerPrePostEvaluationHooks` verifies evaluation phase monitoring
- **Pre/Post Execution Hooks**: `TestEnhancedListenerPrePostExecutionHooks` verifies execution phase monitoring with timing
- **Cycle Hooks**: `TestEnhancedListenerCycleHooks` verifies cycle-level monitoring with data context
- **Mixed Listeners**: `TestMixedListeners` confirms both basic and enhanced listeners work together
- **Error Handling**: `TestEnhancedListenerErrorHandling` validates error capture in both phases

All tests follow TDD principles with:
- Proper use of `require` and `assert` from testify
- Subtests with descriptive "should" naming
- Parallel execution where appropriate
- Helper functions for test setup
- Focused, deterministic test scenarios

## Migration

No breaking changes - enhancement is fully opt-in:

- Existing `GruleEngineListener` implementations continue to work unchanged
- New implementations can use `EnhancedGruleEngineListener` for additional capabilities
- Both listener types can be mixed in the same engine
- Interface detection happens at runtime - no registration required

## Use Cases

This enhancement enables:
- **Performance Monitoring**: Precise rule evaluation and execution timing
- **Advanced Debugging**: Detailed execution traces with data context inspection
- **Shadow Mode Testing**: Non-intrusive rule evaluation for A/B testing
- **Compliance Auditing**: Complete audit trails with data context snapshots
- **Custom Metrics**: Application-specific rule execution analytics
- **Error Analytics**: Comprehensive error tracking and analysis
- **Data Lineage**: Track how data flows through rule evaluations
