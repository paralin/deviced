package jsonmessage

type JSONError struct {
	Code    int    "code,omitempty"
	Message string "message,omitempty"
}

func (e *JSONError) Error() string {
	return e.Message
}
