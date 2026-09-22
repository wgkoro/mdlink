package diagnostic

type Diagnostic struct {
	RawLink    string   `json:"raw_link,omitempty"`
	Fragment   string   `json:"fragment,omitempty"`
	Target     string   `json:"target,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	Phase      string   `json:"phase,omitempty"`
	Code       string   `json:"code"`
	Source     string   `json:"source"`
	Offset     *int     `json:"offset,omitempty"`
	RawTarget  *string  `json:"raw_target,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
}
