package engine

import (
	"context"
	"testing"
	"time"

	"github.com/hyperjumptech/grule-rule-engine/ast"
	"github.com/hyperjumptech/grule-rule-engine/builder"
	"github.com/hyperjumptech/grule-rule-engine/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	listenerRules = `
rule TestRule "triggers side effect in when clause and sleeps in then clause" salience 10 {
	when 
		fact.Value == "initial"
	then 
		fact.TriggerSideEffect();
		fact.Sleep();
		fact.Value = "changed";
}
`
	// Rules for testing retraction and deletion
	retractRule = `
rule RetractRule "rule to be retracted" salience 20 {
	when 
		fact.Value == "initial"
	then 
		fact.Value = "should never be executed";
}
`
)

// TestFact represents a test fact for listener testing
type TestFact struct {
	Value            string
	SideEffectCalled bool
}

// TriggerSideEffect simulates a side effect during rule evaluation
func (tf *TestFact) TriggerSideEffect() bool {
	tf.SideEffectCalled = true
	return true
}

// Sleep simulates a sleep operation with configurable duration
func (tf *TestFact) Sleep() {
	// For testing purposes, we'll just record that sleep was called
	// without actually sleeping for the full duration
	time.Sleep(10 * time.Millisecond) // Short sleep for testing
}

// TestEnhancedListener implements EnhancedGruleEngineListener for testing
type TestEnhancedListener struct {
	// Track method calls
	PreEvaluateCalls  []ListenerCall
	PostEvaluateCalls []ListenerCall
	PreExecuteCalls   []ListenerCall
	PostExecuteCalls  []ListenerCall
	BeginCycleCalls   []CycleCall

	// Track execution timing
	executionTimers map[string]time.Time
	executionTimes  map[string]time.Duration

	// Configuration
	shouldRetractRule bool
	retractedRules    []string

	// Configuration for rule manipulation
	preEvaluationRulesToRetract []string
	preExecutionRulesToRetract  []string
}

// ListenerCall represents a method call on the listener
type ListenerCall struct {
	Cycle     uint64
	RuleName  string
	Candidate *bool // nil for non-evaluation calls
	Error     error
	Fact      *TestFact
}

// CycleCall represents a BeginCycle call
type CycleCall struct {
	Cycle uint64
	Fact  *TestFact
}

func NewTestEnhancedListener() *TestEnhancedListener {
	return &TestEnhancedListener{
		PreEvaluateCalls:            make([]ListenerCall, 0),
		PostEvaluateCalls:           make([]ListenerCall, 0),
		PreExecuteCalls:             make([]ListenerCall, 0),
		PostExecuteCalls:            make([]ListenerCall, 0),
		BeginCycleCalls:             make([]CycleCall, 0),
		executionTimers:             make(map[string]time.Time),
		executionTimes:              make(map[string]time.Duration),
		retractedRules:              make([]string, 0),
		preEvaluationRulesToRetract: make([]string, 0),
	}
}

// Implementation of GruleEngineListener interface
func (tel *TestEnhancedListener) EvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool) {
	// Basic listener method - should be called for compatibility
}

func (tel *TestEnhancedListener) ExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry) {
	// Basic listener method - should be called for compatibility
}

func (tel *TestEnhancedListener) BeginCycle(ctx context.Context, cycle uint64) {
	// Basic listener method - should be called for compatibility
}

// Implementation of EnhancedGruleEngineListener interface
func (tel *TestEnhancedListener) PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	fact := dataCtx.Get("fact").Value().Interface().(*TestFact)
	tel.PreEvaluateCalls = append(tel.PreEvaluateCalls, ListenerCall{
		Cycle:    cycle,
		RuleName: entry.RuleName,
		Fact:     fact,
	})

	// Check if this rule should be retracted
	for _, ruleName := range tel.preEvaluationRulesToRetract {
		if entry.RuleName == ruleName {
			entry.Retracted = true
		}
	}
}

func (tel *TestEnhancedListener) PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error) {
	fact := dataCtx.Get("fact").Value().Interface().(*TestFact)
	tel.PostEvaluateCalls = append(tel.PostEvaluateCalls, ListenerCall{
		Cycle:     cycle,
		RuleName:  entry.RuleName,
		Candidate: &candidate,
		Error:     execError,
		Fact:      fact,
	})
}

func (tel *TestEnhancedListener) PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext) {
	ruleName := entry.RuleName
	fact := dataCtx.Get("fact").Value().Interface().(*TestFact)
	tel.PreExecuteCalls = append(tel.PreExecuteCalls, ListenerCall{
		Cycle:    cycle,
		RuleName: ruleName,
		Fact:     fact,
	})

	// Start timing the execution
	tel.executionTimers[ruleName] = time.Now()

	// Optionally retract rule if configured
	if tel.shouldRetractRule {
		entry.Retracted = true
		tel.retractedRules = append(tel.retractedRules, ruleName)
	}

	for _, ruleName := range tel.preExecutionRulesToRetract {
		if entry.RuleName == ruleName {
			entry.Retracted = true
			tel.retractedRules = append(tel.retractedRules, ruleName)
		}
	}
}

func (tel *TestEnhancedListener) PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error) {
	ruleName := entry.RuleName
	fact := dataCtx.Get("fact").Value().Interface().(*TestFact)
	tel.PostExecuteCalls = append(tel.PostExecuteCalls, ListenerCall{
		Cycle:    cycle,
		RuleName: ruleName,
		Error:    execError,
		Fact:     fact,
	})

	// Calculate execution time
	if startTime, exists := tel.executionTimers[ruleName]; exists {
		tel.executionTimes[ruleName] = time.Since(startTime)
		delete(tel.executionTimers, ruleName)
	}
}

func (tel *TestEnhancedListener) BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext) {
	tel.BeginCycleCalls = append(tel.BeginCycleCalls, CycleCall{
		Cycle: cycle,
		Fact:  dataCtx.Get("fact").Value().Interface().(*TestFact),
	})
}

func TestEnhancedGruleEngineListener(t *testing.T) {
	t.Parallel()

	t.Run("should call all enhanced listener methods during rule execution", func(t *testing.T) {
		t.Parallel()
		// Given: a test fact and enhanced listener
		fact := &TestFact{Value: "initial"}
		listener := NewTestEnhancedListener()

		// And: a data context with the fact
		dctx := ast.NewDataContext()
		err := dctx.Add("fact", fact)
		require.NoError(t, err, "Failed to add fact to data context")

		// And: a knowledge base with test rules
		kb := mustCreateKnowledgeBase(t, listenerRules)

		// And: an engine with the enhanced listener
		engine := NewGruleEngine()
		engine.Listeners = append(engine.Listeners, listener)

		// When: executing the rules
		err = engine.Execute(dctx, kb)

		// Then: no execution error should occur
		require.NoError(t, err, "Rule execution should not fail")

		// And: all enhanced listener methods should be called
		assert.NotEmpty(t, listener.BeginCycleCalls, "BeginCycleWithContext should be called")
		assert.NotEmpty(t, listener.PreEvaluateCalls, "PreEvaluateRuleEntry should be called")
		assert.NotEmpty(t, listener.PostEvaluateCalls, "PostEvaluateRuleEntry should be called")
		assert.NotEmpty(t, listener.PreExecuteCalls, "PreExecuteRuleEntry should be called")
		assert.NotEmpty(t, listener.PostExecuteCalls, "PostExecuteRuleEntry should be called")

		// And: the side effect should have been triggered
		assert.True(t, fact.SideEffectCalled, "Side effect should have been called during evaluation")

		// And: the fact value should have been changed during execution
		assert.Equal(t, "changed", fact.Value, "Fact value should have been changed during rule execution")
	})

	t.Run("should provide access to data context in all methods", func(t *testing.T) {
		t.Parallel()
		// Given: a test fact and enhanced listener
		fact := &TestFact{Value: "initial"}
		listener := NewTestEnhancedListener()

		// And: a data context with the fact
		dctx := ast.NewDataContext()
		err := dctx.Add("fact", fact)
		require.NoError(t, err)

		// And: a knowledge base with test rules
		kb := mustCreateKnowledgeBase(t, listenerRules)

		// And: an engine with the enhanced listener
		engine := NewGruleEngine()
		engine.Listeners = append(engine.Listeners, listener)

		// When: executing the rules
		err = engine.Execute(dctx, kb)
		require.NoError(t, err)

		// Then: all listener calls should have access to data context
		for _, call := range listener.PreEvaluateCalls {
			assert.NotNil(t, call.Fact, "PreEvaluateRuleEntry should have access to fact")
		}

		for _, call := range listener.PostEvaluateCalls {
			assert.NotNil(t, call.Fact, "PostEvaluateRuleEntry should have access to fact")
		}

		for _, call := range listener.PreExecuteCalls {
			assert.NotNil(t, call.Fact, "PreExecuteRuleEntry should have access to fact")
		}

		for _, call := range listener.PostExecuteCalls {
			assert.NotNil(t, call.Fact, "PostExecuteRuleEntry should have access to fact")
		}

		for _, call := range listener.BeginCycleCalls {
			assert.NotNil(t, call.Fact, "BeginCycleWithContext should have access to fact")
		}
	})

	t.Run("should measure rule execution time between pre and post execute calls", func(t *testing.T) {
		// Given: a test fact and enhanced listener
		fact := &TestFact{Value: "initial"}
		listener := NewTestEnhancedListener()

		// And: a data context with the fact
		dctx := ast.NewDataContext()
		err := dctx.Add("fact", fact)
		require.NoError(t, err)

		// And: a knowledge base with test rules
		kb := mustCreateKnowledgeBase(t, listenerRules)

		// And: an engine with the enhanced listener
		engine := NewGruleEngine()
		engine.Listeners = append(engine.Listeners, listener)

		// When: executing the rules
		err = engine.Execute(dctx, kb)
		require.NoError(t, err)

		// Then: execution time should be recorded for the rule
		assert.Contains(t, listener.executionTimes, "TestRule", "Execution time should be recorded for TestRule")

		executionTime := listener.executionTimes["TestRule"]
		assert.Greater(t, executionTime, time.Duration(0), "Execution time should be greater than 0")
		assert.Less(t, executionTime, 1*time.Second, "Execution time should be reasonable (less than 1 second)")
	})

	t.Run("should be able to retract rules during execution", func(t *testing.T) {
		// Given: a test fact and enhanced listener configured to retract rules
		fact := &TestFact{Value: "initial"}
		listener := NewTestEnhancedListener()
		listener.shouldRetractRule = true

		// And: a data context with the fact
		dctx := ast.NewDataContext()
		err := dctx.Add("fact", fact)
		require.NoError(t, err)

		// And: a knowledge base with test rules
		kb := mustCreateKnowledgeBase(t, listenerRules)

		// And: an engine with the enhanced listener
		engine := NewGruleEngine()
		engine.Listeners = append(engine.Listeners, listener)

		// When: executing the rules
		err = engine.Execute(dctx, kb)
		require.NoError(t, err)

		// Then: the rule should have been retracted
		assert.NotEmpty(t, listener.retractedRules, "At least one rule should have been retracted")
		assert.Contains(t, listener.retractedRules, "TestRule", "TestRule should have been retracted")

		// And: the fact value should remain unchanged since rule was retracted before execution
		assert.Equal(t, "initial", fact.Value, "Fact value should remain unchanged as rule was retracted")
	})

	t.Run("should retract rules before evaluation", func(t *testing.T) {
		// Given: a test fact and enhanced listener
		fact := &TestFact{Value: "initial"}
		listener := NewTestEnhancedListener()
		listener.preEvaluationRulesToRetract = []string{"RetractRule"}

		// And: a data context with the fact
		dctx := ast.NewDataContext()
		err := dctx.Add("fact", fact)
		require.NoError(t, err)

		// And: a knowledge base with multiple rules
		kb := mustCreateKnowledgeBase(t, retractRule)

		// And: an engine with the enhanced listener
		engine := NewGruleEngine()
		engine.Listeners = append(engine.Listeners, listener)

		// When: executing the rules
		err = engine.Execute(dctx, kb)
		require.NoError(t, err)

		// Then: validate that the rule was called in PreEvaluate but not evaluated
		assert.Len(t, listener.PreEvaluateCalls, 1, "PreEvaluateRuleEntry should have been called")
		assert.Equal(t, "RetractRule", listener.PreEvaluateCalls[0].RuleName, "PreEvaluateRuleEntry should have been called for RetractRule")

		// And: the rule should not have been evaluated (no PostEvaluate calls)
		assert.Empty(t, listener.PostEvaluateCalls, "rule should not have been evaluated after retraction")

		// And: the fact value should remain unchanged
		assert.Equal(t, "initial", fact.Value, "Fact value should remain unchanged as rule was retracted")
	})

	t.Run("should retract rules before execution", func(t *testing.T) {
		// Given: a test fact and enhanced listener
		fact := &TestFact{Value: "initial"}
		listener := NewTestEnhancedListener()
		listener.preExecutionRulesToRetract = []string{"RetractRule"}

		// And: a data context with the fact
		dctx := ast.NewDataContext()
		err := dctx.Add("fact", fact)
		require.NoError(t, err)

		// And: a knowledge base with multiple rules
		kb := mustCreateKnowledgeBase(t, retractRule)

		// And: an engine with the enhanced listener
		engine := NewGruleEngine()
		engine.Listeners = append(engine.Listeners, listener)

		// When: executing the rules
		err = engine.Execute(dctx, kb)
		require.NoError(t, err)

		// Then: validate that the rule was evaluated but not executed
		assert.Len(t, listener.PreEvaluateCalls, 1, "PreEvaluateRuleEntry should have been called")
		assert.Len(t, listener.PostEvaluateCalls, 1, "PostEvaluateRuleEntry should have been called")
		assert.Len(t, listener.PreExecuteCalls, 1, "PreExecuteRuleEntry should have been called")
		assert.Empty(t, listener.PostExecuteCalls, "rule should not have been executed after retraction")

		// And: the fact value should remain unchanged
		assert.Equal(t, "initial", fact.Value, "Fact value should remain unchanged as rule was retracted")
	})
}

// mustCreateKnowledgeBase is a helper function to create a knowledge base for testing
func mustCreateKnowledgeBase(t *testing.T, rules string) *ast.KnowledgeBase {
	t.Helper()

	lib := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(lib)
	err := rb.BuildRuleFromResource("Test", "0.1.1", pkg.NewBytesResource([]byte(rules)))
	require.NoError(t, err, "Failed to build rules")

	kb, err := lib.NewKnowledgeBaseInstance("Test", "0.1.1")
	require.NoError(t, err, "Failed to create knowledge base instance")

	return kb
}
