package check

import (
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/config"
)

// on is how a test writes a rule with its switch either way.
func on(enabled bool) *bool { return &enabled }

// rule is a rule with one action, so the configuration passes validation.
func rule(name string, when ...config.Condition) config.Rule {
	return config.Rule{Name: name, When: when, MoveTo: "somewhere"}
}

// warnings returns only the shadowing findings, which is what the analysis is.
func warnings(problems []Problem) []Problem {
	var out []Problem
	for _, p := range problems {
		if p.Severity == Warning {
			out = append(out, p)
		}
	}
	return out
}

func TestAMistakeIsAnError(t *testing.T) {
	cfg := config.Config{
		Conflict: config.ConflictRename,
		Rules: []config.Rule{{
			Name:   "bad",
			When:   []config.Condition{{Ext: []string{"pdf"}}},
			MoveTo: "../outside",
		}},
	}
	problems := Rules(cfg)
	if len(problems) != 1 || problems[0].Severity != Error {
		t.Fatalf("got %+v", problems)
	}
	if !strings.Contains(problems[0].Message, "climbs out") {
		t.Errorf("message = %q", problems[0].Message)
	}
	if problems[0].Rule != "bad" {
		t.Errorf("rule = %q, want the rule the problem belongs to", problems[0].Rule)
	}
	if !HasErrors(problems) {
		t.Error("HasErrors said no")
	}
}

func TestAValidConfigurationHasNothingToSay(t *testing.T) {
	problems := Rules(config.Default("en"))
	if len(problems) != 0 {
		t.Errorf("the built-in rules were reported: %+v", problems)
	}
}

func TestAShadowedRuleIsAWarning(t *testing.T) {
	cases := map[string]struct {
		earlier config.Rule
		later   config.Rule
		shadow  bool
	}{
		"same conditions": {
			rule("a", config.Condition{Ext: []string{"pdf"}}),
			rule("b", config.Condition{Ext: []string{"pdf"}}),
			true,
		},
		"narrower extension list": {
			rule("a", config.Condition{Ext: []string{"pdf"}}),
			rule("b", config.Condition{Ext: []string{"pdf", "docx"}}),
			true,
		},
		"wider extension list does not shadow": {
			rule("a", config.Condition{Ext: []string{"pdf", "docx"}}),
			rule("b", config.Condition{Ext: []string{"pdf"}}),
			false,
		},
		"more conditions shadow fewer": {
			rule("a",
				config.Condition{Category: "image"},
				config.Condition{NameContains: []string{"Screenshot"}},
			),
			rule("b", config.Condition{Category: "image"}),
			true,
		},
		"fewer conditions do not shadow more": {
			rule("a", config.Condition{Category: "image"}),
			rule("b",
				config.Condition{Category: "image"},
				config.Condition{NameContains: []string{"Screenshot"}},
			),
			false,
		},
		"a size floor shadows a lower one": {
			rule("a", config.Condition{LargerThan: "10MB"}),
			rule("b", config.Condition{LargerThan: "1MB"}),
			true,
		},
		"a lower size floor does not": {
			rule("a", config.Condition{LargerThan: "1MB"}),
			rule("b", config.Condition{LargerThan: "10MB"}),
			false,
		},
		"recent days shadow a longer window": {
			rule("a", config.Condition{ModifiedWithinDays: days(7)}),
			rule("b", config.Condition{ModifiedWithinDays: days(30)}),
			true,
		},
		"a longer window does not shadow a shorter one": {
			rule("a", config.Condition{ModifiedWithinDays: days(30)}),
			rule("b", config.Condition{ModifiedWithinDays: days(7)}),
			false,
		},
		"the later rule needs only one condition": {
			config.Rule{Name: "a", When: []config.Condition{{Ext: []string{"png"}}}, MoveTo: "x"},
			config.Rule{
				Name:   "b",
				Match:  "any",
				When:   []config.Condition{{Ext: []string{"png"}}, {Ext: []string{"jpg"}}},
				MoveTo: "x",
			},
			true,
		},
		"different kinds of condition say nothing": {
			rule("a", config.Condition{Category: "document"}),
			rule("b", config.Condition{Ext: []string{"pdf"}}),
			false,
		},
		"opposite polarity says nothing": {
			rule("a", config.Condition{Not: true, Ext: []string{"pdf"}}),
			rule("b", config.Condition{Ext: []string{"pdf"}}),
			false,
		},
	}
	for name, c := range cases {
		cfg := config.Config{Conflict: config.ConflictRename, Rules: []config.Rule{c.earlier, c.later}}
		got := warnings(Rules(cfg))
		if c.shadow && len(got) != 1 {
			t.Errorf("%s: expected a warning, got %+v", name, got)
		}
		if !c.shadow && len(got) != 0 {
			t.Errorf("%s: expected silence, got %+v", name, got)
		}
	}
}

func TestContinueMeansTheLaterRuleStillRuns(t *testing.T) {
	earlier := rule("a", config.Condition{Ext: []string{"pdf"}})
	earlier.Continue = true
	cfg := config.Config{
		Conflict: config.ConflictRename,
		Rules:    []config.Rule{earlier, rule("b", config.Condition{Ext: []string{"pdf"}})},
	}
	if got := warnings(Rules(cfg)); len(got) != 0 {
		t.Errorf("a rule that says continue does not shadow: %+v", got)
	}
}

func TestADisabledRuleShadowsNothingAndIsNotReported(t *testing.T) {
	earlier := rule("a", config.Condition{Ext: []string{"pdf"}})
	earlier.Enabled = on(false)
	cfg := config.Config{
		Conflict: config.ConflictRename,
		Rules:    []config.Rule{earlier, rule("b", config.Condition{Ext: []string{"pdf"}})},
	}
	if got := warnings(Rules(cfg)); len(got) != 0 {
		t.Errorf("a disabled rule shadows nothing: %+v", got)
	}

	later := rule("b", config.Condition{Ext: []string{"pdf"}})
	later.Enabled = on(false)
	cfg.Rules = []config.Rule{rule("a", config.Condition{Ext: []string{"pdf"}}), later}
	if got := warnings(Rules(cfg)); len(got) != 0 {
		t.Errorf("a disabled rule is not worth a warning: %+v", got)
	}
}

func TestTheMessageSaysWhatToDo(t *testing.T) {
	cfg := config.Config{
		Conflict: config.ConflictRename,
		Rules: []config.Rule{
			rule("PDFs", config.Condition{Ext: []string{"pdf"}}),
			rule("Documents", config.Condition{Ext: []string{"pdf", "docx"}}),
		},
	}
	got := warnings(Rules(cfg))
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	for _, want := range []string{`"Documents"`, `"PDFs"`, "continue = true"} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("message does not mention %q: %s", want, got[0].Message)
		}
	}
}

func TestAShadowedRuleIsStillOnlyAWarning(t *testing.T) {
	cfg := config.Config{
		Conflict: config.ConflictRename,
		Rules: []config.Rule{
			rule("a", config.Condition{Ext: []string{"pdf"}}),
			rule("b", config.Condition{Ext: []string{"pdf"}}),
		},
	}
	if HasErrors(Rules(cfg)) {
		t.Error("a shadowed rule must not stop the program: the configuration is legal")
	}
}

func days(n int) *int { return &n }
