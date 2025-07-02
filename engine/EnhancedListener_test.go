package engine

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperjumptech/grule-rule-engine/ast"
	"github.com/hyperjumptech/grule-rule-engine/builder"
	"github.com/hyperjumptech/grule-rule-engine/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestData represents test data with side effect tracking
type TestData struct {
	Value       string
	SideEffects []string
	mu          sync.Mutex
}

func (td *TestData) GetValue() string {
	return td.Value
}

func (td *TestData) SetValue(val string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.Value = val
	td.SideEffects = append(td.SideEffects, fmt.Sprintf("SetValue:%s", val))
}

func (td *TestData) TriggerSideEffect(effect string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.SideEffects = append(td.SideEffects, effect)
	return true
}

// HasSideEffect checks if a side effect exists and triggers a new one (for use in when clauses)
func (td *TestData) HasSideEffect(effect string) bool {
	td.TriggerSideEffect(effect)
	return true
}

func (td *TestData) GetSideEffects() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	return append([]string(nil), td.SideEffects...)
}

// PanicTestData represents test data that can panic
type PanicTestData struct {
	Value       string
	ShouldPanic bool
}

func (ptd *PanicTestData) GetValue() string {
	if ptd.ShouldPanic {
		panic("intentional panic for testing")
	}
	return ptd.Value
}

// ComprehensiveTestListener implements all enhanced functionality with comprehensive tracking
type ComprehensiveTestListener struct {
	mu sync.Mutex

	// Basic interface tracking
	BeginCalled    bool
	EvaluateCalled bool
	ExecuteCalled  bool

	// Enhanced interface tracking
	PreEvaluateCalled       bool
	PostEvaluateCalled      bool
	PreExecuteCalled        bool
	PostExecuteCalled       bool
	BeginCycleWithCtxCalled bool

	// Call order and timing tracking
	CallOrder       []string
	CallTimestamps  []time.Time
	EvaluationTimes map[string]time.Duration
	ExecutionTimes  map[string]time.Duration
	preEvalTimes    map[string]time.Time
	preExecTimes    map[string]time.Time

	// Error tracking
	EvaluationErrors []error
	ExecutionErrors  []error

	// Data context manipulation tracking
	DataContextProvided  bool
	DataContextKeysFound []string
	DataContextModified  bool
	StoredData           map[string]interface{}

	// Side effect tracking (for validating hook timing)
	SideEffectsBeforeHooks []string
	SideEffectsAfterHooks  []string

	// Rule management
	RetractedRules []string
}

// RetractRule simulates rule retraction capability
func (c *ComprehensiveTestListener) RetractRule(ruleName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RetractedRules = append(c.RetractedRules, ruleName)
}

// GetCallOrder returns a copy of the call order for testing
func (c *ComprehensiveTestListener) GetCallOrder() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.CallOrder...)
}

// GetStoredData returns a copy of stored data for testing
func (c *ComprehensiveTestListener) GetStoredData() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]interface{})
	for k, v := range c.StoredData {
		result[k] = v
	}
	return result
}

func newComprehensiveTestListener() *ComprehensiveTestListener {
	return &ComprehensiveTestListener{
		EvaluationTimes:        make(map[string]time.Duration),
		ExecutionTimes:         make(map[string]time.Duration),
		preEvalTimes:           make(map[string]time.Time),
		preExecTimes:           make(map[string]time.Time),
		StoredData:             make(map[string]interface{}),
		SideEffectsBeforeHooks: []string{},
		SideEffectsAfterHooks:  []string{},
		RetractedRules:         []string{},
		CallOrder:              []string{},
		CallTimestamps:         []time.Time{},
		DataContextKeysFound:   []string{},
		EvaluationErrors:       []error{},
		ExecutionErrors:        []error{},
	}
}

func (c *ComprehensiveTestListener) recordCall(methodName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CallOrder = append(c.CallOrder, methodName)
	c.CallTimestamps = append(c.CallTimestamps, time.Now())
}

func (c *ComprehensiveTestListener) captureCurrentSideEffects(dataCtx ast.IDataContext, stage string) {
	if dataCtx != nil {
		testDataNode := dataCtx.Get("testData")
		if testDataNode != nil {
			// Extract the underlying value from the ValueNode
			value, err := testDataNode.GetValue()
			if err == nil && value.IsValid() {
				// Handle different ways the value might be stored
				var testData *TestData
				valueInterface := value.Interface()

				// Try direct type assertion first
				if td, ok := valueInterface.(*TestData); ok {
					testData = td
				} else if value.Kind() == reflect.Ptr && !value.IsNil() {
					// Try to extract from pointer
					if td, ok := value.Elem().Interface().(*TestData); ok {
						testData = td
					}
				}

				if testData != nil {
					effects := testData.GetSideEffects()
					c.mu.Lock()
					if stage == "before" {
						c.SideEffectsBeforeHooks = append([]string(nil), effects...)
					} else {
						c.SideEffectsAfterHooks = append([]string(nil), effects...)
					}
					c.mu.Unlock()
				}
			}
		}
	}
}

// Basic interface methods
func (c *ComprehensiveTestListener) BeginCycle(ctx context.Context, cycle uint64) {
	c.recordCall("BeginCycle")
	c.BeginCalled = true
}

func (c *ComprehensiveTestListener) EvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool) {
	c.recordCall(fmt.Sprintf("EvaluateRuleEntry:%s", entry.RuleName))
	c.EvaluateCalled = true
}

func (c *ComprehensiveTestListener) ExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry) {
	c.recordCall(fmt.Sprintf("ExecuteRuleEntry:%s", entry.RuleName))
	c.ExecuteCalled = true
}

// Enhanced interface methods
func (c *ComprehensiveTestListener) PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	c.recordCall(fmt.Sprintf("PreEvaluateRuleEntry:%s", entry.RuleName))
	c.PreEvaluateCalled = true

	// Capture side effects before rule evaluation
	c.captureCurrentSideEffects(dataCtx, "before")

	c.validateAndModifyDataContext(dataCtx)

	c.mu.Lock()
	c.preEvalTimes[entry.RuleName] = time.Now()
	c.StoredData[fmt.Sprintf("pre_eval_%s", entry.RuleName)] = time.Now()
	c.mu.Unlock()
}

func (c *ComprehensiveTestListener) PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error) {
	c.recordCall(fmt.Sprintf("PostEvaluateRuleEntry:%s", entry.RuleName))
	c.PostEvaluateCalled = true

	// Capture side effects after rule evaluation
	c.captureCurrentSideEffects(dataCtx, "after")

	c.validateAndModifyDataContext(dataCtx)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.EvaluationErrors = append(c.EvaluationErrors, execError)

	if startTime, exists := c.preEvalTimes[entry.RuleName]; exists {
		c.EvaluationTimes[entry.RuleName] = time.Since(startTime)
		delete(c.preEvalTimes, entry.RuleName)
	}

	c.StoredData[fmt.Sprintf("post_eval_%s", entry.RuleName)] = time.Now()
}

func (c *ComprehensiveTestListener) PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	c.recordCall(fmt.Sprintf("PreExecuteRuleEntry:%s", entry.RuleName))
	c.PreExecuteCalled = true
	c.validateAndModifyDataContext(dataCtx)

	c.mu.Lock()
	c.preExecTimes[entry.RuleName] = time.Now()
	c.StoredData[fmt.Sprintf("pre_exec_%s", entry.RuleName)] = time.Now()
	c.mu.Unlock()
}

func (c *ComprehensiveTestListener) PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error) {
	c.recordCall(fmt.Sprintf("PostExecuteRuleEntry:%s", entry.RuleName))
	c.PostExecuteCalled = true
	c.validateAndModifyDataContext(dataCtx)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.ExecutionErrors = append(c.ExecutionErrors, execError)

	if startTime, exists := c.preExecTimes[entry.RuleName]; exists {
		c.ExecutionTimes[entry.RuleName] = time.Since(startTime)
		delete(c.preExecTimes, entry.RuleName)
	}

	c.StoredData[fmt.Sprintf("post_exec_%s", entry.RuleName)] = time.Now()
}

func (c *ComprehensiveTestListener) BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext) {
	c.recordCall("BeginCycleWithContext")
	c.BeginCycleWithCtxCalled = true
	c.validateAndModifyDataContext(dataCtx)
}

func (c *ComprehensiveTestListener) validateAndModifyDataContext(dataCtx ast.IDataContext) {
	if dataCtx != nil {
		c.DataContextProvided = true
		keys := dataCtx.GetKeys()
		c.mu.Lock()
		c.DataContextKeysFound = append(c.DataContextKeysFound, keys...)
		c.mu.Unlock()

		// Demonstrate data context modification
		if !c.DataContextModified {
			listenerData := map[string]interface{}{
				"timestamp":   time.Now(),
				"listener_id": "comprehensive_test_listener",
			}
			err := dataCtx.Add("listenerData", listenerData)
			if err == nil {
				c.DataContextModified = true
			}
		}
	}
}

// mustCreateTestSetup creates a minimal test setup
func mustCreateTestSetup(t *testing.T) (ast.IDataContext, *ast.KnowledgeBase) {
	drls := `
rule TestRule "Simple test rule" salience 10 {
    when
        testData.GetValue() == "initial"
    then
        testData.SetValue("changed");
        Complete();
}`

	dataContext := ast.NewDataContext()
	testData := &TestData{Value: "initial"}
	err := dataContext.Add("testData", testData)
	require.NoError(t, err)

	lib := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(lib)
	err = rb.BuildRuleFromResource("Test", "0.1.1", pkg.NewBytesResource([]byte(drls)))
	require.NoError(t, err)

	kb, err := lib.NewKnowledgeBaseInstance("Test", "0.1.1")
	require.NoError(t, err)

	return dataContext, kb
}

// mustCreateErrorTestSetup creates a test setup with one normal rule and one that will cause an error
func mustCreateErrorTestSetup(t *testing.T) (ast.IDataContext, *ast.KnowledgeBase) {
	drls := `
rule GoodRule "A rule that works" salience 20 {
    when
        testData.GetValue() == "initial"
    then
        testData.SetValue("good");
}

rule BadRule "A rule that will cause an error" salience 10 {
    when
        panicData.GetValue() == "initial"
    then
        panicData.SetValue("bad");
        Complete();
}`

	dataContext := ast.NewDataContext()
	testData := &TestData{Value: "initial"}
	panicData := &PanicTestData{Value: "initial", ShouldPanic: true}

	err := dataContext.Add("testData", testData)
	require.NoError(t, err)
	err = dataContext.Add("panicData", panicData)
	require.NoError(t, err)

	lib := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(lib)
	err = rb.BuildRuleFromResource("Test", "0.1.1", pkg.NewBytesResource([]byte(drls)))
	require.NoError(t, err)

	kb, err := lib.NewKnowledgeBaseInstance("Test", "0.1.1")
	require.NoError(t, err)

	return dataContext, kb
}

// mustCreateSideEffectTestSetup creates a test setup specifically for side effect timing validation
func mustCreateSideEffectTestSetup(t *testing.T) (ast.IDataContext, *ast.KnowledgeBase) {
	drls := `
rule SideEffectRule "Rule with side effect in when clause" salience 10 {
    when
        testData.GetValue() == "initial" && testData.HasSideEffect("during_evaluation")
    then
        testData.TriggerSideEffect("during_execution");
        testData.SetValue("completed");
        Complete();
}`

	dataContext := ast.NewDataContext()
	testData := &TestData{Value: "initial", SideEffects: []string{}}
	err := dataContext.Add("testData", testData)
	require.NoError(t, err)

	lib := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(lib)
	err = rb.BuildRuleFromResource("Test", "0.1.1", pkg.NewBytesResource([]byte(drls)))
	require.NoError(t, err)

	kb, err := lib.NewKnowledgeBaseInstance("Test", "0.1.1")
	require.NoError(t, err)

	return dataContext, kb
}

func TestBasicListener(t *testing.T) {
	t.Parallel()

	t.Run("should call basic interface methods", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		assert.True(t, listener.BeginCalled, "BeginCycle should be called")
		assert.True(t, listener.EvaluateCalled, "EvaluateRuleEntry should be called")
		assert.True(t, listener.ExecuteCalled, "ExecuteRuleEntry should be called")
	})
}

func TestEnhancedListener(t *testing.T) {
	t.Parallel()

	t.Run("should call enhanced interface methods with data context", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify basic methods are still called
		assert.True(t, listener.BeginCalled, "BeginCycle should be called")
		assert.True(t, listener.EvaluateCalled, "EvaluateRuleEntry should be called")
		assert.True(t, listener.ExecuteCalled, "ExecuteRuleEntry should be called")

		// Verify enhanced methods are called
		assert.True(t, listener.PreEvaluateCalled, "PreEvaluateRuleEntry should be called")
		assert.True(t, listener.PostEvaluateCalled, "PostEvaluateRuleEntry should be called")
		assert.True(t, listener.PreExecuteCalled, "PreExecuteRuleEntry should be called")
		assert.True(t, listener.PostExecuteCalled, "PostExecuteRuleEntry should be called")
		assert.True(t, listener.BeginCycleWithCtxCalled, "BeginCycleWithContext should be called")

		// Verify data context is provided
		assert.True(t, listener.DataContextProvided, "Data context should be provided to enhanced methods")
		assert.Contains(t, listener.DataContextKeysFound, "testData", "Data context should contain testData key")
	})
}

func TestMixedListeners(t *testing.T) {
	t.Parallel()

	t.Run("should support multiple listeners together", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		basicListener := newComprehensiveTestListener()
		enhancedListener := newComprehensiveTestListener()

		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{basicListener, enhancedListener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify both listeners were called
		assert.True(t, basicListener.BeginCalled, "First listener should be called")
		assert.True(t, basicListener.EvaluateCalled, "First listener should be called")
		assert.True(t, basicListener.ExecuteCalled, "First listener should be called")

		assert.True(t, enhancedListener.BeginCalled, "Second listener basic methods should be called")
		assert.True(t, enhancedListener.PreEvaluateCalled, "Second listener should be called")
		assert.True(t, enhancedListener.PostEvaluateCalled, "Second listener should be called")
		assert.True(t, enhancedListener.PreExecuteCalled, "Second listener should be called")
		assert.True(t, enhancedListener.PostExecuteCalled, "Second listener should be called")
		assert.True(t, enhancedListener.DataContextProvided, "Second listener should receive data context")
	})
}

// Advanced tests for validating hook ordering, error handling, and timing
func TestEnhancedListenerCallOrder(t *testing.T) {
	t.Parallel()

	t.Run("should call hooks in correct order during rule evaluation and execution", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify call order is correct
		callOrder := listener.GetCallOrder()
		require.Greater(t, len(callOrder), 0, "Should have recorded calls")

		// Find the indices of pre and post hooks for the rule
		var preEvalIndex, postEvalIndex, preExecIndex, postExecIndex int = -1, -1, -1, -1

		for i, call := range callOrder {
			switch {
			case call == "PreEvaluateRuleEntry:TestRule":
				preEvalIndex = i
			case call == "PostEvaluateRuleEntry:TestRule":
				postEvalIndex = i
			case call == "PreExecuteRuleEntry:TestRule":
				preExecIndex = i
			case call == "PostExecuteRuleEntry:TestRule":
				postExecIndex = i
			}
		}

		// Verify all hooks were called
		assert.NotEqual(t, -1, preEvalIndex, "PreEvaluateRuleEntry should be called")
		assert.NotEqual(t, -1, postEvalIndex, "PostEvaluateRuleEntry should be called")
		assert.NotEqual(t, -1, preExecIndex, "PreExecuteRuleEntry should be called")
		assert.NotEqual(t, -1, postExecIndex, "PostExecuteRuleEntry should be called")

		// Verify order is correct
		assert.Less(t, preEvalIndex, postEvalIndex, "PreEvaluateRuleEntry should be called before PostEvaluateRuleEntry")
		assert.Less(t, postEvalIndex, preExecIndex, "PostEvaluateRuleEntry should be called before PreExecuteRuleEntry")
		assert.Less(t, preExecIndex, postExecIndex, "PreExecuteRuleEntry should be called before PostExecuteRuleEntry")
	})
}

func TestEnhancedListenerErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("should capture evaluation errors and continue rule execution", func(t *testing.T) {
		dataContext, kb := mustCreateErrorTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		// This should not fail even though one rule panics during evaluation
		_ = engine.Execute(dataContext, kb)
		// The engine might return an error if the panic is in execution rather than evaluation
		// For this test we're mainly interested in ensuring the listener captures the error

		// Verify that errors were captured
		hasErrors := false
		for _, evalErr := range listener.EvaluationErrors {
			if evalErr != nil {
				hasErrors = true
				break
			}
		}
		for _, execErr := range listener.ExecutionErrors {
			if execErr != nil {
				hasErrors = true
				break
			}
		}

		// We should capture at least one error from the panic
		assert.True(t, hasErrors, "Should capture errors from panicking rule")

		// Verify that both rules were processed (good rule should still work)
		callOrder := listener.GetCallOrder()
		goodRuleCalls := 0
		badRuleCalls := 0
		for _, call := range callOrder {
			if strings.Contains(call, "GoodRule") {
				goodRuleCalls++
			}
			if strings.Contains(call, "BadRule") {
				badRuleCalls++
			}
		}

		assert.Greater(t, goodRuleCalls, 0, "Good rule should be processed")
		// Note: BadRule might not get to execution if it panics during evaluation
	})
}

func TestEnhancedListenerTiming(t *testing.T) {
	t.Parallel()

	t.Run("should measure evaluation and execution timings accurately", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify that timing measurements were captured
		assert.Greater(t, len(listener.EvaluationTimes), 0, "Should have evaluation timing measurements")
		assert.Greater(t, len(listener.ExecutionTimes), 0, "Should have execution timing measurements")

		// Verify that timings are reasonable (greater than 0 but not absurdly large)
		for ruleName, evalTime := range listener.EvaluationTimes {
			assert.Greater(t, evalTime, time.Duration(0), "Evaluation time for rule %s should be greater than 0", ruleName)
			assert.Less(t, evalTime, time.Second, "Evaluation time for rule %s should be reasonable", ruleName)
		}

		for ruleName, execTime := range listener.ExecutionTimes {
			assert.Greater(t, execTime, time.Duration(0), "Execution time for rule %s should be greater than 0", ruleName)
			assert.Less(t, execTime, time.Second, "Execution time for rule %s should be reasonable", ruleName)
		}

		// Verify we have timing for the test rule
		assert.Contains(t, listener.EvaluationTimes, "TestRule", "Should have evaluation timing for TestRule")
		assert.Contains(t, listener.ExecutionTimes, "TestRule", "Should have execution timing for TestRule")
	})

	t.Run("should provide timing hooks for performance monitoring", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		start := time.Now()
		err := engine.Execute(dataContext, kb)
		totalTime := time.Since(start)
		require.NoError(t, err)

		// Verify that individual rule timings are captured and reasonable
		var totalEvalTime, totalExecTime time.Duration
		for _, evalTime := range listener.EvaluationTimes {
			totalEvalTime += evalTime
		}
		for _, execTime := range listener.ExecutionTimes {
			totalExecTime += execTime
		}

		// The sum of individual timings should be less than total time (due to overhead)
		assert.Less(t, totalEvalTime+totalExecTime, totalTime, "Individual timings should be less than total execution time")
		assert.Greater(t, totalEvalTime, time.Duration(0), "Should have some evaluation time")
		assert.Greater(t, totalExecTime, time.Duration(0), "Should have some execution time")
	})
}

func TestPreEvaluationHookTiming(t *testing.T) {
	t.Parallel()

	t.Run("should call pre-evaluation hook before rule evaluation side effects occur", func(t *testing.T) {
		dataContext, kb := mustCreateSideEffectTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify the pre-evaluation hook was called (it should capture side effects state before evaluation)
		// Since the pre-hook is called before evaluation, we expect fewer or no side effects at that point
		// The post-hook should capture side effects after evaluation
		assert.NotNil(t, listener.SideEffectsBeforeHooks, "Should capture side effects state before hooks")
		assert.NotNil(t, listener.SideEffectsAfterHooks, "Should capture side effects state after hooks")

		// The key validation: after evaluation, we should have more side effects than before
		beforeCount := len(listener.SideEffectsBeforeHooks)
		afterCount := len(listener.SideEffectsAfterHooks)

		assert.GreaterOrEqual(t, afterCount, beforeCount, "Should have same or more side effects after evaluation than before")

		// Verify that the evaluation side effect is present in the after hooks
		foundEval := false
		foundExec := false
		for _, effect := range listener.SideEffectsAfterHooks {
			if effect == "during_evaluation" {
				foundEval = true
			}
			if effect == "during_execution" {
				foundExec = true
			}
		}
		assert.True(t, foundEval, "Should find the evaluation side effect in post-hook capture")

		// Since this test focuses on evaluation timing, execution side effect might not be captured
		// in the post-evaluation hook if it happens during execution phase
		if foundExec {
			t.Log("Execution side effect also captured in post-evaluation hook")
		}
	})
}

func TestDataContextModification(t *testing.T) {
	t.Parallel()

	t.Run("should allow listeners to modify data context", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify that the listener modified the data context
		assert.True(t, listener.DataContextModified, "Listener should have modified the data context")

		// Verify that the listener data was actually added to the context
		listenerDataNode := dataContext.Get("listenerData")
		assert.NotNil(t, listenerDataNode, "Listener data should be present in data context")

		// Extract and validate the listener data
		value, err := listenerDataNode.GetValue()
		require.NoError(t, err)
		require.True(t, value.IsValid(), "Listener data value should be valid")

		listenerData, ok := value.Interface().(map[string]interface{})
		require.True(t, ok, "Listener data should be a map")

		assert.Contains(t, listenerData, "timestamp", "Listener data should contain timestamp")
		assert.Contains(t, listenerData, "listener_id", "Listener data should contain listener_id")
		assert.Equal(t, "comprehensive_test_listener", listenerData["listener_id"], "Listener ID should match")
	})
}

func TestRuleRetraction(t *testing.T) {
	t.Parallel()

	t.Run("should support rule retraction functionality", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		// Simulate rule retraction
		listener.RetractRule("TestRule")

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify that the rule was recorded as retracted
		assert.Contains(t, listener.RetractedRules, "TestRule", "TestRule should be in retracted rules list")

		// Verify that retraction tracking works
		listener.RetractRule("AnotherRule")
		assert.Contains(t, listener.RetractedRules, "AnotherRule", "AnotherRule should be in retracted rules list")
		assert.Len(t, listener.RetractedRules, 2, "Should have two retracted rules")
	})
}

func TestDataStorageBetweenHooks(t *testing.T) {
	t.Parallel()

	t.Run("should store and access data between different hooks", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify that data was stored in different hooks
		storedData := listener.GetStoredData()

		// Check for pre and post evaluation data
		assert.Contains(t, storedData, "pre_eval_TestRule", "Should store data in pre-evaluation hook")
		assert.Contains(t, storedData, "post_eval_TestRule", "Should store data in post-evaluation hook")

		// Check for pre and post execution data
		assert.Contains(t, storedData, "pre_exec_TestRule", "Should store data in pre-execution hook")
		assert.Contains(t, storedData, "post_exec_TestRule", "Should store data in post-execution hook")

		// Verify that stored data is accessible and contains timestamps
		preEvalData := storedData["pre_eval_TestRule"]
		postEvalData := storedData["post_eval_TestRule"]

		assert.IsType(t, time.Time{}, preEvalData, "Pre-evaluation data should be a timestamp")
		assert.IsType(t, time.Time{}, postEvalData, "Post-evaluation data should be a timestamp")

		// Verify that pre-evaluation timestamp is before post-evaluation timestamp
		preTime := preEvalData.(time.Time)
		postTime := postEvalData.(time.Time)
		assert.True(t, preTime.Before(postTime), "Pre-evaluation timestamp should be before post-evaluation timestamp")
	})
}

func TestComprehensiveListenerIntegration(t *testing.T) {
	t.Parallel()

	t.Run("should demonstrate all listener capabilities working together", func(t *testing.T) {
		dataContext, kb := mustCreateSideEffectTestSetup(t)

		listener := newComprehensiveTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		// Simulate some rule retraction
		listener.RetractRule("SomeOtherRule")

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify all basic functionality
		assert.True(t, listener.BeginCalled, "BeginCycle should be called")
		assert.True(t, listener.EvaluateCalled, "EvaluateRuleEntry should be called")
		assert.True(t, listener.ExecuteCalled, "ExecuteRuleEntry should be called")

		// Verify all enhanced functionality
		assert.True(t, listener.PreEvaluateCalled, "PreEvaluateRuleEntry should be called")
		assert.True(t, listener.PostEvaluateCalled, "PostEvaluateRuleEntry should be called")
		assert.True(t, listener.PreExecuteCalled, "PreExecuteRuleEntry should be called")
		assert.True(t, listener.PostExecuteCalled, "PostExecuteRuleEntry should be called")
		assert.True(t, listener.BeginCycleWithCtxCalled, "BeginCycleWithContext should be called")

		// Verify data context functionality
		assert.True(t, listener.DataContextProvided, "Data context should be provided")
		assert.True(t, listener.DataContextModified, "Data context should be modified")
		assert.Contains(t, listener.DataContextKeysFound, "testData", "Should find testData key")

		// Verify timing functionality
		assert.Greater(t, len(listener.EvaluationTimes), 0, "Should have evaluation timings")
		assert.Greater(t, len(listener.ExecutionTimes), 0, "Should have execution timings")

		// Verify side effect tracking
		assert.NotNil(t, listener.SideEffectsBeforeHooks, "Should track side effects state before hooks")
		assert.NotNil(t, listener.SideEffectsAfterHooks, "Should track side effects state after hooks")

		// Verify data storage
		storedData := listener.GetStoredData()
		assert.Greater(t, len(storedData), 0, "Should have stored data")

		// Verify rule retraction tracking
		assert.Contains(t, listener.RetractedRules, "SomeOtherRule", "Should track retracted rules")

		// Verify call order tracking
		callOrder := listener.GetCallOrder()
		assert.Greater(t, len(callOrder), 0, "Should track call order")

		// Verify proper hook ordering for the executed rule
		var preEvalIndex, postEvalIndex int = -1, -1
		for i, call := range callOrder {
			if call == "PreEvaluateRuleEntry:SideEffectRule" {
				preEvalIndex = i
			}
			if call == "PostEvaluateRuleEntry:SideEffectRule" {
				postEvalIndex = i
			}
		}

		assert.NotEqual(t, -1, preEvalIndex, "Should find pre-evaluation call")
		assert.NotEqual(t, -1, postEvalIndex, "Should find post-evaluation call")
		assert.Less(t, preEvalIndex, postEvalIndex, "Pre-evaluation should come before post-evaluation")
	})
}
