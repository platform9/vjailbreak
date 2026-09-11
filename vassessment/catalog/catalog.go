package catalog

var registry []Rule

// Register adds a rule to the catalog. Called from init() in each rules
// source file — never edit this file to add a rule.
func Register(r Rule) {
	registry = append(registry, r)
}

// All returns every registered rule, in registration order.
func All() []Rule {
	out := make([]Rule, len(registry))
	copy(out, registry)
	return out
}

// Evaluate runs every registered rule against vm.
func Evaluate(vm VM) []Result {
	results := make([]Result, 0, len(registry))
	for _, r := range registry {
		results = append(results, r.Evaluate(vm))
	}
	return results
}
