package codex

import (
	"strings"
	"testing"

	"github.com/zeeshans/shugoshin/internal/types"
)

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name     string
		intent   string
		diffs    map[string]string
		contains []string
	}{
		{
			name:   "single file",
			intent: "fix the null pointer bug in user.go",
			diffs: map[string]string{
				"internal/user/user.go": "-\treturn user\n+\tif user == nil { return nil, ErrNotFound }\n+\treturn user, nil",
			},
			contains: []string{
				"fix the null pointer bug in user.go",
				"internal/user/user.go",
				"-\treturn user",
				"YOUR ANALYSIS TASKS:",
				"1. Identify every function",
				"2. Search the codebase",
				"3. Reason about whether",
				"4. Check if any shared utilities",
				"5. Assess whether the changes",
				"Respond ONLY with a JSON object matching the provided schema.",
			},
		},
		{
			name:   "multiple files",
			intent: "refactor auth middleware",
			diffs: map[string]string{
				"middleware/auth.go":   "+func NewMiddleware() *Middleware {",
				"middleware/token.go":  "-func Validate(t string) bool {",
			},
			contains: []string{
				"refactor auth middleware",
				"middleware/auth.go",
				"middleware/token.go",
				"CHANGED FILES AND DIFFS:",
				"TASK INTENT:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPrompt(tt.intent, tt.diffs)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildPrompt() output missing %q\nfull output:\n%s", want, got)
				}
			}
		})
	}
}

func TestBuildPrompt_EmptyDiffs(t *testing.T) {
	intent := "add unit tests"
	got := BuildPrompt(intent, map[string]string{})

	mustContain := []string{
		intent,
		"TASK INTENT:",
		"CHANGED FILES AND DIFFS:",
		"YOUR ANALYSIS TASKS:",
		"Respond ONLY with a JSON object matching the provided schema.",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("BuildPrompt() with empty diffs missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want types.Verdict
	}{
		{
			name: "safe verdict",
			raw: `{
				"verdict": "SAFE",
				"summary": "No blast radius detected",
				"affected_areas": [],
				"intent_match": true,
				"reasoning": "The changes are isolated and backward compatible."
			}`,
			want: types.Verdict{
				Verdict:       "SAFE",
				Summary:       "No blast radius detected",
				AffectedAreas: []types.AffectedArea{},
				IntentMatch:   true,
				Reasoning:     "The changes are isolated and backward compatible.",
			},
		},
		{
			name: "review needed with affected areas",
			raw: `{
				"verdict": "REVIEW_NEEDED",
				"summary": "Three call sites may be affected",
				"affected_areas": [
					{
						"symbol": "GetUser()",
						"locations": ["api/handlers/user.go:42", "api/handlers/auth.go:87"],
						"risk": "MEDIUM"
					}
				],
				"intent_match": true,
				"reasoning": "GetUser() signature changed; callers need review."
			}`,
			want: types.Verdict{
				Verdict: "REVIEW_NEEDED",
				Summary: "Three call sites may be affected",
				AffectedAreas: []types.AffectedArea{
					{
						Symbol:    "GetUser()",
						Locations: []string{"api/handlers/user.go:42", "api/handlers/auth.go:87"},
						Risk:      "MEDIUM",
					},
				},
				IntentMatch: true,
				Reasoning:   "GetUser() signature changed; callers need review.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVerdict([]byte(tt.raw))
			if err != nil {
				t.Fatalf("parseVerdict() returned unexpected error: %v", err)
			}
			if got.Verdict != tt.want.Verdict {
				t.Errorf("Verdict = %q, want %q", got.Verdict, tt.want.Verdict)
			}
			if got.Summary != tt.want.Summary {
				t.Errorf("Summary = %q, want %q", got.Summary, tt.want.Summary)
			}
			if got.IntentMatch != tt.want.IntentMatch {
				t.Errorf("IntentMatch = %v, want %v", got.IntentMatch, tt.want.IntentMatch)
			}
			if got.Reasoning != tt.want.Reasoning {
				t.Errorf("Reasoning = %q, want %q", got.Reasoning, tt.want.Reasoning)
			}
			if len(got.AffectedAreas) != len(tt.want.AffectedAreas) {
				t.Fatalf("len(AffectedAreas) = %d, want %d", len(got.AffectedAreas), len(tt.want.AffectedAreas))
			}
			for i, area := range got.AffectedAreas {
				wantArea := tt.want.AffectedAreas[i]
				if area.Symbol != wantArea.Symbol {
					t.Errorf("AffectedAreas[%d].Symbol = %q, want %q", i, area.Symbol, wantArea.Symbol)
				}
				if area.Risk != wantArea.Risk {
					t.Errorf("AffectedAreas[%d].Risk = %q, want %q", i, area.Risk, wantArea.Risk)
				}
				if len(area.Locations) != len(wantArea.Locations) {
					t.Errorf("AffectedAreas[%d] len(Locations) = %d, want %d", i, len(area.Locations), len(wantArea.Locations))
				}
			}
		})
	}
}

func TestParseVerdict_InvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty input", raw: ""},
		{name: "plain text", raw: "not json at all"},
		{name: "partial json", raw: `{"verdict": "SAFE"`},
		{name: "wrong type", raw: `["SAFE", "summary"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVerdict([]byte(tt.raw))
			if err != nil {
				t.Fatalf("parseVerdict() returned unexpected Go error: %v", err)
			}
			if got.Verdict != "ERROR" {
				t.Errorf("Verdict = %q, want %q", got.Verdict, "ERROR")
			}
			if got.Summary != "Invalid JSON from Codex" {
				t.Errorf("Summary = %q, want %q", got.Summary, "Invalid JSON from Codex")
			}
			if got.Reasoning != tt.raw {
				t.Errorf("Reasoning = %q, want raw input %q", got.Reasoning, tt.raw)
			}
		})
	}
}
