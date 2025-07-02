package engine

import (
	"context"
	"fmt"
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

// TestData represents simple test data
type TestData struct {
	Value string
}

func (td *TestData) GetValue() string {
	return td.Value
}

func (td *TestData) SetValue(val string) {
	td.Value = val
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

// MinimalTestListener implements only the basic interface
type MinimalTestListener struct {
	EvaluateCalled bool
	ExecuteCalled  bool
	BeginCalled    bool
}

func (m *MinimalTestListener) EvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool) {
	m.EvaluateCalled = true
}

func (m *MinimalTestListener) ExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry) {
	m.ExecuteCalled = true
}

func (m *MinimalTestListener) BeginCycle(ctx context.Context, cycle uint64) {
	m.BeginCalled = true
}

// TimingAndErrorTestListener implements enhanced interface with comprehensive tracking
type TimingAndErrorTestListener struct {
	MinimalTestListener
	mu sync.Mutex

	// Call order tracking
	CallOrder []string

	// Enhanced methods tracking
	PreEvaluateCalled       bool
	PostEvaluateCalled      bool
	PreExecuteCalled        bool
	PostExecuteCalled       bool
	BeginCycleWithCtxCalled bool

	// Error tracking
	EvaluationErrors []error
	ExecutionErrors  []error

	// Timing tracking
	EvaluationTimes map[string]time.Duration
	ExecutionTimes  map[string]time.Duration
	preEvalTimes    map[string]time.Time
	preExecTimes    map[string]time.Time

	// Data context validation
	DataContextProvided  bool
	DataContextKeysFound []string
}

func newTimingAndErrorTestListener() *TimingAndErrorTestListener {
	return &TimingAndErrorTestListener{
		EvaluationTimes: make(map[string]time.Duration),
		ExecutionTimes:  make(map[string]time.Duration),
		preEvalTimes:    make(map[string]time.Time),
		preExecTimes:    make(map[string]time.Time),
	}
}

func (t *TimingAndErrorTestListener) recordCall(methodName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CallOrder = append(t.CallOrder, methodName)
}

func (t *TimingAndErrorTestListener) BeginCycle(ctx context.Context, cycle uint64) {
	t.recordCall("BeginCycle")
	t.BeginCalled = true
}

func (t *TimingAndErrorTestListener) EvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool) {
	t.recordCall(fmt.Sprintf("EvaluateRuleEntry:%s", entry.RuleName))
	t.EvaluateCalled = true
}

func (t *TimingAndErrorTestListener) ExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry) {
	t.recordCall(fmt.Sprintf("ExecuteRuleEntry:%s", entry.RuleName))
	t.ExecuteCalled = true
}

func (t *TimingAndErrorTestListener) PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	t.recordCall(fmt.Sprintf("PreEvaluateRuleEntry:%s", entry.RuleName))
	t.PreEvaluateCalled = true
	t.validateDataContext(dataCtx)

	t.mu.Lock()
	t.preEvalTimes[entry.RuleName] = time.Now()
	t.mu.Unlock()
}

func (t *TimingAndErrorTestListener) PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error) {
	t.recordCall(fmt.Sprintf("PostEvaluateRuleEntry:%s", entry.RuleName))
	t.PostEvaluateCalled = true
	t.validateDataContext(dataCtx)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Record error if present
	t.EvaluationErrors = append(t.EvaluationErrors, execError)

	// Calculate timing if pre-evaluation was recorded
	if startTime, exists := t.preEvalTimes[entry.RuleName]; exists {
		t.EvaluationTimes[entry.RuleName] = time.Since(startTime)
		delete(t.preEvalTimes, entry.RuleName)
	}
}

func (t *TimingAndErrorTestListener) PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	t.recordCall(fmt.Sprintf("PreExecuteRuleEntry:%s", entry.RuleName))
	t.PreExecuteCalled = true
	t.validateDataContext(dataCtx)

	t.mu.Lock()
	t.preExecTimes[entry.RuleName] = time.Now()
	t.mu.Unlock()
}

func (t *TimingAndErrorTestListener) PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error) {
	t.recordCall(fmt.Sprintf("PostExecuteRuleEntry:%s", entry.RuleName))
	t.PostExecuteCalled = true
	t.validateDataContext(dataCtx)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Record error if present
	t.ExecutionErrors = append(t.ExecutionErrors, execError)

	// Calculate timing if pre-execution was recorded
	if startTime, exists := t.preExecTimes[entry.RuleName]; exists {
		t.ExecutionTimes[entry.RuleName] = time.Since(startTime)
		delete(t.preExecTimes, entry.RuleName)
	}
}

func (t *TimingAndErrorTestListener) BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext) {
	t.recordCall("BeginCycleWithContext")
	t.BeginCycleWithCtxCalled = true
	t.validateDataContext(dataCtx)
}

func (t *TimingAndErrorTestListener) validateDataContext(dataCtx ast.IDataContext) {
	if dataCtx != nil {
		t.DataContextProvided = true
		keys := dataCtx.GetKeys()
		t.mu.Lock()
		t.DataContextKeysFound = append(t.DataContextKeysFound, keys...)
		t.mu.Unlock()
	}
}

// EnhancedTestListener implements the enhanced interface
type EnhancedTestListener struct {
	MinimalTestListener

	// Enhanced methods tracking
	PreEvaluateCalled       bool
	PostEvaluateCalled      bool
	PreExecuteCalled        bool
	PostExecuteCalled       bool
	BeginCycleWithCtxCalled bool

	// Data context validation
	DataContextProvided  bool
	DataContextKeysFound []string
}

func (e *EnhancedTestListener) PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	e.PreEvaluateCalled = true
	e.validateDataContext(dataCtx)
}

func (e *EnhancedTestListener) PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error) {
	e.PostEvaluateCalled = true
	e.validateDataContext(dataCtx)
}

func (e *EnhancedTestListener) PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	e.PreExecuteCalled = true
	e.validateDataContext(dataCtx)
}

func (e *EnhancedTestListener) PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error) {
	e.PostExecuteCalled = true
	e.validateDataContext(dataCtx)
}

func (e *EnhancedTestListener) BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext) {
	e.BeginCycleWithCtxCalled = true
	e.validateDataContext(dataCtx)
}

func (e *EnhancedTestListener) validateDataContext(dataCtx ast.IDataContext) {
	if dataCtx != nil {
		e.DataContextProvided = true
		keys := dataCtx.GetKeys()
		e.DataContextKeysFound = append(e.DataContextKeysFound, keys...)
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

func TestBasicListener(t *testing.T) {
	t.Parallel()

	t.Run("should call basic interface methods", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := &MinimalTestListener{}
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

		listener := &EnhancedTestListener{}
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

	t.Run("should support both basic and enhanced listeners together", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		basicListener := &MinimalTestListener{}
		enhancedListener := &EnhancedTestListener{}

		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{basicListener, enhancedListener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify both listeners were called
		assert.True(t, basicListener.BeginCalled, "Basic listener should be called")
		assert.True(t, basicListener.EvaluateCalled, "Basic listener should be called")
		assert.True(t, basicListener.ExecuteCalled, "Basic listener should be called")

		assert.True(t, enhancedListener.BeginCalled, "Enhanced listener basic methods should be called")
		assert.True(t, enhancedListener.PreEvaluateCalled, "Enhanced listener should be called")
		assert.True(t, enhancedListener.PostEvaluateCalled, "Enhanced listener should be called")
		assert.True(t, enhancedListener.PreExecuteCalled, "Enhanced listener should be called")
		assert.True(t, enhancedListener.PostExecuteCalled, "Enhanced listener should be called")
		assert.True(t, enhancedListener.DataContextProvided, "Enhanced listener should receive data context")
	})
}

// Advanced tests for validating hook ordering, error handling, and timing
func TestEnhancedListenerCallOrder(t *testing.T) {
	t.Parallel()

	t.Run("should call hooks in correct order during rule evaluation and execution", func(t *testing.T) {
		dataContext, kb := mustCreateTestSetup(t)

		listener := newTimingAndErrorTestListener()
		engine := NewGruleEngine()
		engine.Listeners = []GruleEngineListener{listener}

		err := engine.Execute(dataContext, kb)
		require.NoError(t, err)

		// Verify call order is correct
		require.Greater(t, len(listener.CallOrder), 0, "Should have recorded calls")

		// Find the indices of pre and post hooks for the rule
		var preEvalIndex, postEvalIndex, preExecIndex, postExecIndex int = -1, -1, -1, -1

		for i, call := range listener.CallOrder {
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

		listener := newTimingAndErrorTestListener()
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
		goodRuleCalls := 0
		badRuleCalls := 0
		for _, call := range listener.CallOrder {
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

		listener := newTimingAndErrorTestListener()
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

		listener := newTimingAndErrorTestListener()
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
