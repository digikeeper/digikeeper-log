package errs

import "errors"

// wrappedErr is an immutable sentinel that chains to a parent via Unwrap.
// Prefer this over fmt.Errorf("...: %w", parent) for package-level declarations.
type wrappedErr struct {
	msg    string
	parent error
}

func (e *wrappedErr) Error() string { return e.msg }
func (e *wrappedErr) Unwrap() error { return e.parent }

func wrap(msg string, parent error) error {
	return &wrappedErr{msg, parent}
}

// Base categories.
var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("conflict")
	ErrCommonDomain = errors.New("domain rule violation")
	ErrCommonInfra  = errors.New("infra related")
)

// Specific business errors. Each wraps its category so both are detectable via errors.Is.
var (
	ErrEntryNotFound       = wrap("entry not found", ErrNotFound)
	ErrCandidateNotPending = wrap("candidate not pending", ErrNotFound)

	ErrUnknownAction = wrap("unknown resolution action", ErrInvalidInput)

	ErrIncompleteResolution = wrap("all pending candidates must be resolved at once", ErrCommonDomain)
	ErrMultipleApply        = wrap("at most one candidate per entry may be applied", ErrCommonDomain)
)

// Infrastructure errors.
var (
	ErrStorageCommon = wrap("storage common error", ErrCommonInfra)
	ErrIndexFailed   = wrap("index failed", ErrCommonInfra)
)
