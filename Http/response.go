package Http

import "strings"

type Response struct {
	body    []byte
	code    uint8
	headers *Headers
	cookies *Cookies
}

func NewResponse(code uint8, body []byte, headers *Headers, cookies *Cookies) *Response {
	return &Response{
		body:    body,
		code:    code,
		headers: headers,
		cookies: cookies,
	}
}

func (r *Response) GetHeaders() *Headers {
	return r.headers
}

func (r *Response) GetCookies() *Cookies {
	return r.cookies
}

func (r *Response) IsSuccessful() bool {
	return r.code == 200
}

func (r *Response) GetContentType() string {
	var value string

	if r.headers.Has("Content-Type") {
		value = r.headers.Get("Content-Type")
		if strings.Contains(value, ";") {
			value, _, _ = strings.Cut(value, ";")
		}
	}

	return value
}
