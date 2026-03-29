package errs

import "errors"

var (
	StorageCommonError = errors.New("storage common error")
	IndexFailed        = errors.New("index failed")
	EntryNotFound      = errors.New("entry not found")
)
