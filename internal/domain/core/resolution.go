package core

type CandidateResolution string

const (
	Apply CandidateResolution = "apply"
	Deny  CandidateResolution = "deny"
)

func (a CandidateResolution) IsValid() bool {
	return a == Apply || a == Deny
}
