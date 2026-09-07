// Package catalog implements vAssessment's pre-check rule catalog (PRD §6):
// a versioned set of rules, each a pure function from a VM's discovered
// data to a pass/fail verdict-impact result.
//
// Rules register themselves via init() in per-source-file (e.g. rules_src.go,
// later rules_manual.go, rules_dst.go) so adding, changing, or retiring a
// single rule never touches the others.
package catalog

// VerdictImpact is what a failed rule contributes to a VM's overall
// readiness verdict (PRD §5.2, Pillar C).
type VerdictImpact string

const (
	VerdictInformational   VerdictImpact = "informational"
	VerdictReadyWithSteps  VerdictImpact = "ready_with_steps"
	VerdictNeedsAssessment VerdictImpact = "needs_assessment"
	VerdictBlocked         VerdictImpact = "blocked"
)

// Result is one rule's outcome against one VM.
type Result struct {
	RuleID   string
	Passed   bool
	Verdict  VerdictImpact // impact when Passed is false; zero value when Passed is true
	Evidence string        // the observed value(s) that produced the verdict
}

// Rule is one catalog entry. Tier follows the PRD's data-coverage ladder:
// "t1" (creds only) through "t5" (destination).
type Rule struct {
	ID       string
	Name     string
	Category string
	Tier     string
	Evaluate func(vm VM) Result
}
