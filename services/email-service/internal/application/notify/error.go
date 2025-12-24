package notify

type TemporaryError struct{ msg string }

func (e TemporaryError) Error() string   { return e.msg }
func (e TemporaryError) Temporary() bool { return true }
func (e TemporaryError) Permanent() bool { return false }

type PermanentError struct{ msg string }

func (e PermanentError) Error() string   { return e.msg }
func (e PermanentError) Permanent() bool { return true }
