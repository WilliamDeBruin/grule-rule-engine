package engine

import (
	"context"

	"github.com/hyperjumptech/grule-rule-engine/ast"
)

// GruleEngineListener is an interface to be implemented by those who want to listen the Engine execution.
type GruleEngineListener interface {
	// EvaluateRuleEntry will be called by the engine if it evaluate a rule entry
	EvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool)
	// ExecuteRuleEntry will be called by the engine if it execute a rule entry in a cycle
	ExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry)
	// BeginCycle will be called by the engine every time it start a new evaluation cycle
	BeginCycle(ctx context.Context, cycle uint64)
}

// EnhancedGruleEngineListener extends GruleEngineListener with additional hooks for data context access
// and pre and post rule evaluation and execution hooks. Listeners can implement either interface - the engine will detect
// which interface is implemented and call the appropriate methods.
type EnhancedGruleEngineListener interface {
	GruleEngineListener

	// PreEvaluateRuleEntry will be called by the engine before it evaluates a rule entry
	PreEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext)

	// PostEvaluateRuleEntry will be called by the engine after it evaluates a rule entry.
	PostEvaluateRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, candidate bool, dataCtx ast.IDataContext, execError error)

	// PreExecuteRuleEntry will be called immediately before a rule entry is executed
	PreExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext)

	// PostExecuteRuleEntry will be called immediately after a rule entry is executed
	PostExecuteRuleEntry(ctx context.Context, cycle uint64, entry *ast.RuleEntry, dataCtx ast.IDataContext, execError error)

	// BeginCycleWithContext will be called by the engine every time it starts a new evaluation cycle,
	// providing access to the data context.
	BeginCycleWithContext(ctx context.Context, cycle uint64, dataCtx ast.IDataContext)
}
