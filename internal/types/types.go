package types

import "time"

type HookPayload struct {
	SessionID        string                 `json:"session_id"`
	HookEventName    string                 `json:"hook_event_name"`
	Cwd              string                 `json:"cwd"`
	Prompt           string                 `json:"prompt,omitempty"`
	ToolName         string                 `json:"tool_name,omitempty"`
	ToolInput        map[string]interface{} `json:"tool_input,omitempty"`
	ToolResponse     map[string]interface{} `json:"tool_response,omitempty"`
	StopHookActive   bool                   `json:"stop_hook_active"`
	TranscriptPath   string                 `json:"transcript_path,omitempty"`
	LastAssistantMsg string                 `json:"last_assistant_message,omitempty"`
}

type SessionState struct {
	SessionID      string   `json:"session_id"`
	Cwd            string   `json:"cwd"`
	CurrentIntent  string   `json:"current_intent"`
	CurrentChanges []string `json:"current_changes"`
	ResponseIndex  int      `json:"response_index"`
}

type AffectedArea struct {
	Symbol    string   `json:"symbol"`
	Locations []string `json:"locations"`
	Risk      string   `json:"risk"`
}

type Verdict struct {
	Verdict       string         `json:"verdict"`
	Summary       string         `json:"summary"`
	AffectedAreas []AffectedArea `json:"affected_areas"`
	IntentMatch   bool           `json:"intent_match"`
	Reasoning     string         `json:"reasoning"`
}

type Report struct {
	SessionID     string   `json:"session_id"`
	Cwd           string   `json:"cwd"`
	Timestamp     time.Time `json:"timestamp"`
	ResponseIndex int      `json:"response_index"`
	Intent        string   `json:"intent"`
	ChangedFiles  []string `json:"changed_files"`
	Verdict       Verdict  `json:"verdict"`
}
