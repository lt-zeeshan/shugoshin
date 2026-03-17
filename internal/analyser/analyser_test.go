package analyser

import (
	"strings"
	"testing"

	"github.com/lt-zeeshan/shugoshin/internal/types"
)

func TestNew(t *testing.T) {
	tests := []struct {
		backend  string
		wantName string
	}{
		{"claude", "claude"},
		{"codex", "codex"},
		{"", "claude"},
		{"unknown", "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			a := New(tt.backend)
			if a.Name() != tt.wantName {
				t.Errorf("New(%q).Name() = %q, want %q", tt.backend, a.Name(), tt.wantName)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name         string
		intent       string
		changedFiles []string
		contains     []string
	}{
		{
			name:         "single file",
			intent:       "fix the null pointer bug in user.go",
			changedFiles: []string{"internal/user/user.go"},
			contains: []string{
				"fix the null pointer bug in user.go",
				"internal/user/user.go",
				"git diff HEAD -- <file>",
				"STEPS:",
				"BEHAVIORAL REGRESSIONS",
				"UNINTENDED SIDE EFFECTS",
				"OPERATIONAL AND RUNTIME RISKS",
				"INTENT DRIFT",
				"TEST GAPS",
				"DO NOT waste time verifying that function signatures",
				"Respond ONLY with a JSON object matching the provided schema.",
			},
		},
		{
			name:         "multiple files",
			intent:       "refactor auth middleware",
			changedFiles: []string{"middleware/auth.go", "middleware/token.go"},
			contains: []string{
				"refactor auth middleware",
				"middleware/auth.go",
				"middleware/token.go",
				"CHANGED FILES:",
				"TASK INTENT:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPrompt(tt.intent, tt.changedFiles)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildPrompt() output missing %q\nfull output:\n%s", want, got)
				}
			}
		})
	}
}

func TestBuildPrompt_EmptyFiles(t *testing.T) {
	intent := "add unit tests"
	got := BuildPrompt(intent, nil)

	mustContain := []string{
		intent,
		"TASK INTENT:",
		"CHANGED FILES:",
		"STEPS:",
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
			if got.Summary != "Invalid JSON from analyser" {
				t.Errorf("Summary = %q, want %q", got.Summary, "Invalid JSON from analyser")
			}
			if got.Reasoning != tt.raw {
				t.Errorf("Reasoning = %q, want raw input %q", got.Reasoning, tt.raw)
			}
		})
	}
}

func TestParseClaudeOutput(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantVerdict   string
		wantSummary   string
		wantReasoning string
	}{
		{
			name:          "valid envelope with structured_output",
			raw:           `{"result":"","structured_output":{"verdict":"SAFE","summary":"ok","affected_areas":[],"intent_match":true,"reasoning":"fine"},"is_error":false}`,
			wantVerdict:   "SAFE",
			wantSummary:   "ok",
			wantReasoning: "fine",
		},
		{
			name:          "empty structured_output falls back to result",
			raw:           `{"result":"{\"verdict\":\"REVIEW_NEEDED\",\"summary\":\"check callers\",\"affected_areas\":[],\"intent_match\":false,\"reasoning\":\"signature changed\"}","is_error":false}`,
			wantVerdict:   "REVIEW_NEEDED",
			wantSummary:   "check callers",
			wantReasoning: "signature changed",
		},
		{
			name:          "is_error true returns ERROR verdict with result as reasoning",
			raw:           `{"result":"something went wrong","structured_output":null,"is_error":true}`,
			wantVerdict:   "ERROR",
			wantSummary:   "Claude returned an error",
			wantReasoning: "something went wrong",
		},
		{
			name:          "invalid outer envelope JSON returns ERROR with raw bytes in reasoning",
			raw:           `not valid json at all`,
			wantVerdict:   "ERROR",
			wantSummary:   "Invalid JSON envelope from Claude",
			wantReasoning: "not valid json at all",
		},
		{
			name:          "structured_output contains invalid verdict JSON returns ERROR",
			raw:           `{"result":"","structured_output":"this is not a verdict object","is_error":false}`,
			wantVerdict:   "ERROR",
			wantSummary:   "Invalid JSON from analyser",
			wantReasoning: `"this is not a verdict object"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClaudeOutput([]byte(tt.raw))
			if err != nil {
				t.Fatalf("parseClaudeOutput() returned unexpected Go error: %v", err)
			}
			if got.Verdict != tt.wantVerdict {
				t.Errorf("Verdict = %q, want %q", got.Verdict, tt.wantVerdict)
			}
			if got.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", got.Summary, tt.wantSummary)
			}
			if got.Reasoning != tt.wantReasoning {
				t.Errorf("Reasoning = %q, want %q", got.Reasoning, tt.wantReasoning)
			}
		})
	}
}
