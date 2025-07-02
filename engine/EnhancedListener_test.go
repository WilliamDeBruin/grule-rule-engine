package engine

import (
	"context"
	"testing"

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

// EnhancedTestListener implements the enhanced interface
type EnhancedTestListener struct {
	MinimalTestListener
	
	// Enhanced methods tracking
	PreEvaluateCalled        bool
	PostEvaluateCalled       bool
	PreExecuteCalled         bool
	PostExecuteCalled        bool
	BeginCycleWithCtxCalled  bool
	
	// Data context validation
	DataContextProvided      bool
	DataContextKeysFound     []string
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
