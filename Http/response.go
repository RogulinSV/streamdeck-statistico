package Http

import (
	"encoding/json"
	"strings"
)

type Response struct {
	body    []byte
	dump    []byte
	code    uint16
	headers *Headers
	cookies *Cookies
}

func NewResponse(code uint16, body []byte, headers *Headers, cookies *Cookies) *Response {
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
	return r.code >= 200 && r.code < 300
}

func (r *Response) IsUnauthorized() bool {
	return r.code == 401
}

func (r *Response) IsForbidden() bool {
	return r.code == 403
}

func (r *Response) GetCode() uint16 {
	return r.code
}

func (r *Response) GetBody() string {
	return string(r.body)
}

func (r *Response) ToJson(proto any) error {
	return json.Unmarshal(r.body, proto)
}

func (r *Response) GetType() string {
	var value string

	if r.headers.Has("Content-Type") {
		value = r.headers.Get("Content-Type")
		if strings.Contains(value, ";") {
			value, _, _ = strings.Cut(value, ";")
		}
	}

	return value
}

func (r *Response) IsJson() bool {
	return strings.Contains(r.GetType(), "application/json")
}
