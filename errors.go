package httpc

type InvalidResource struct {
	err error
}

func (e *InvalidResource) Error() string {
	return "error parsing resource: " + e.err.Error()
}

type RequestError struct {
	err error
}

func (e *RequestError) Error() string {
	return "error making HTTP request: " + e.err.Error()
}

type BadStatusCode struct {
	msg string
}

func (e *BadStatusCode) Error() string {
	return "recieved bad status code: " + e.msg
}

type DecodeError struct {
	err error
}

func (e *DecodeError) Error() string {
	return "failed to decode response body: " + e.err.Error()
}

type CopyError struct {
	err error
}

func (e *CopyError) Error() string {
	return "failed to copy request body: " + e.err.Error()
}
