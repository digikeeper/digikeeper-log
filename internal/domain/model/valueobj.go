package model

// ResolutionAction is an enum for candidate resolution outcomes.
type ResolutionAction string

const (
	Apply ResolutionAction = "apply"
	Deny  ResolutionAction = "deny"
)

func (a ResolutionAction) IsValid() bool {
	return a == Apply || a == Deny
}
