package Http

import "net/http"

type Stack struct {
	request  *http.Request
	response http.ResponseWriter
}

func NewStack(request *http.Request, response http.ResponseWriter) Stack {
	return Stack{
		request:  request,
		response: response,
	}
}

func (stack Stack) GetRequest() *http.Request {
	return stack.request
}

func (stack Stack) GetResponse() http.ResponseWriter {
	return stack.response
}
