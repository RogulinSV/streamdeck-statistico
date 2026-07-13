package Http

import (
	"encoding/base64"
	"encoding/json"
	"iter"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

type Query struct {
	items map[string]string
}

func NewQuery() *Query {
	return &Query{
		items: make(map[string]string),
	}
}

func (q *Query) Set(name string, value string) *Query {
	q.items[name] = value

	return q
}

func (q *Query) Size() int {
	return len(q.items)
}

func (q *Query) String() string {
	var query = make([]string, 0, len(q.items))

	for name, value := range q.items {
		query = append(query, url.QueryEscape(name)+"="+url.QueryEscape(value))
	}

	return strings.Join(query, "&")
}

func (q *Query) Iterate() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for name, value := range q.items {
			if !yield(name, value) {
				return
			}
		}
	}
}

type Data interface {
	Serialize() (string, error)
}

type FormData struct {
	data map[string]string
}

func NewFormData(data map[string]string) *FormData {
	return &FormData{
		data: data,
	}
}

func (d *FormData) Set(name string, value string) *FormData {
	d.data[name] = value

	return d
}

func (d *FormData) Size() int {
	return len(d.data)
}

func (d *FormData) Iterate() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for name, value := range d.data {
			if !yield(name, value) {
				return
			}
		}
	}
}

func (d *FormData) Serialize() (string, error) {
	return "", nil
}

type JsonData struct {
	data any
}

func NewJsonData(data any) *JsonData {
	return &JsonData{
		data: data,
	}
}

func (d *JsonData) Serialize() (string, error) {
	var data, err = json.Marshal(d.data)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

type Files struct {
	items map[string]string
}

func NewFiles() *Files {
	return &Files{
		items: make(map[string]string),
	}
}

func (f *Files) Set(name string, value string) *Files {
	f.items[name] = value

	return f
}

func (f *Files) Size() int {
	return len(f.items)
}

func (f *Files) Iterate() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for name, value := range f.items {
			if !yield(name, value) {
				return
			}
		}
	}
}

type Headers struct {
	items map[string][]string
}

func (h *Headers) Set(name string, value string) *Headers {
	name = textproto.CanonicalMIMEHeaderKey(name)
	h.items[name] = []string{value}

	return h
}

func (h *Headers) Add(name string, value string) *Headers {
	name = textproto.CanonicalMIMEHeaderKey(name)
	if _, ok := h.items[name]; ok {
		h.items[name] = append(h.items[name], value)
	} else {
		h.items[name] = []string{value}
	}

	return h
}

func (h *Headers) Has(name string) bool {
	var _, ok = h.items[textproto.CanonicalMIMEHeaderKey(name)]

	return ok
}

func (h *Headers) Get(name string) string {
	var headers, ok = h.items[textproto.CanonicalMIMEHeaderKey(name)]
	if ok {
		return headers[0]
	}

	return ""
}

func (h *Headers) Size() int {
	var count = 0

	for _, headers := range h.items {
		count += len(headers)
	}

	return count
}

func (h *Headers) Iterate() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for name, headers := range h.items {
			for _, value := range headers {
				if !yield(name, value) {
					return
				}
			}
		}
	}
}

func SetBasicAuthorizationHeader(r Request, login string, password string) Request {
	var auth = base64.StdEncoding.EncodeToString([]byte(login + ":" + password))
	r.GetHeaders().Set("Authorization", "Basic "+auth)

	return r
}

func SetUserAgentHeader(r Request, value string) Request {
	r.GetHeaders().Set("User-Agent", value)

	return r
}

func SetReferrerHeader(r Request, value string) Request {
	r.GetHeaders().Set("Referer", value)

	return r
}

type Cookies struct {
	items map[string]*http.Cookie
}

func (c *Cookies) Set(name string, value string) *Cookies {
	c.items[name] = &http.Cookie{
		Name:        name,
		Value:       value,
		Quoted:      false,
		Path:        "",
		Domain:      "",
		Expires:     time.Time{},
		RawExpires:  "",
		MaxAge:      0,
		Secure:      false,
		HttpOnly:    false,
		SameSite:    0,
		Partitioned: false,
		Raw:         "",
		Unparsed:    nil,
	}

	return c
}

func (c *Cookies) Has(name string) bool {
	var _, ok = c.items[name]

	return ok
}

func (c *Cookies) GetValue(name string) string {
	if cookie, ok := c.items[name]; ok {
		return cookie.Value
	}

	return ""
}

func (c *Cookies) Size() int {
	return len(c.items)
}

func (c *Cookies) Iterate() iter.Seq2[string, *http.Cookie] {
	return func(yield func(string, *http.Cookie) bool) {
		for name, cookie := range c.items {
			if !yield(name, cookie) {
				return
			}
		}
	}
}

type Request interface {
	GetHeaders() *Headers
	GetCookies() *Cookies
}

type GetRequest struct {
	url     string
	query   *Query
	headers *Headers
	cookies *Cookies
}

func NewGetRequest(url string) *GetRequest {
	return &GetRequest{
		url: url,
	}
}

func (r *GetRequest) GetQuery() *Query {
	return r.query
}

func (r *GetRequest) GetHeaders() *Headers {
	return r.headers
}

func (r *GetRequest) GetCookies() *Cookies {
	return r.cookies
}

type PostFormRequest struct {
	*GetRequest
	data  *FormData
	files *Files
}

func NewPostFormRequest(url string, data *FormData, files *Files) *PostFormRequest {
	return &PostFormRequest{
		GetRequest: NewGetRequest(url),
		data:       data,
		files:      files,
	}
}

func (r *PostFormRequest) GetData() *FormData {
	return r.data
}

func (r *PostFormRequest) HasData() bool {
	return r.data.Size() > 0
}

func (r *PostFormRequest) GetFiles() *Files {
	return r.files
}

func (r *PostFormRequest) HasFiles() bool {
	return r.files.Size() > 0
}

type PostJsonRequest struct {
	*GetRequest
	data *JsonData
}

func NewPostJsonRequest(url string, data *JsonData) *PostJsonRequest {
	return &PostJsonRequest{
		GetRequest: NewGetRequest(url),
		data:       data,
	}
}

func (r *PostJsonRequest) GetJson() *JsonData {
	return r.data
}
