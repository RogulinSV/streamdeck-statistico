package Http

import (
	"context"
	"crypto/tls"
	"io"
	"maps"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
)

const (
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 6.2; WOW64) AppleWebKit/537.46 (KHTML, like Gecko) Chrome/50.0.3922.369 Safari/600"
)

type Progress func(uint8, Logger.Logger)

type Context struct {
	progress Progress
	context  context.Context
	secure   bool
	timeout  uint
}

func NewContext(timeout uint, context context.Context) *Context {
	return &Context{
		progress: func(p uint8, l Logger.Logger) {},
		context:  context,
		secure:   true,
		timeout:  timeout,
	}
}

func (c *Context) SetSecure(secure bool) *Context {
	c.secure = secure

	return c
}

func (c *Context) SetContext(context context.Context) *Context {
	c.context = context

	return c
}

func (c *Context) SetProgress(progress Progress) *Context {
	c.progress = progress

	return c
}

func (c *Context) SetTimeout(timeout uint) *Context {
	c.timeout = timeout

	return c
}

type Client struct {
	logger  Logger.Logger
	context *Context
}

func NewClient(context *Context, logger Logger.Logger) *Client {
	return &Client{
		logger:  logger,
		context: context,
	}
}

func (c *Client) WithContext(context *Context) *Client {
	return NewClient(context, c.logger)
}

func (c *Client) Get(r *GetRequest) *Response {
	var method = http.MethodGet
	var request *http.Request
	var uri string
	var err error

	uri = getQuery(r.url, r.query, c.logger)
	if uri == "" {
		return nil
	}

	request, err = http.NewRequestWithContext(c.context.context, method, uri, nil)
	if err != nil {
		c.logger.Error("Не удалось подготовить GET-запрос: {error}", Logger.Context{
			"error": err,
		})
		return nil
	}

	setHeaders(request, r.GetHeaders(), c.logger)
	setCookies(request, r.GetCookies(), c.logger)

	return c.send(request)
}

func (c *Client) Post(r *PostRequest) *Response {
	var method = http.MethodPost
	var reader *strings.Reader
	var request *http.Request
	var data Data
	var serialized string
	var uri string
	var err error

	uri = getQuery(r.url, r.query, c.logger)
	if uri == "" {
		return nil
	}

	data = r.GetData()
	serialized, err = data.Serialize()
	if err != nil {
		c.logger.Error("Не удалось преобразовать {type}-данные в строку: {error}", Logger.Context{
			"type":  data.GetType(),
			"error": err,
		})
		return nil
	}

	reader = strings.NewReader(serialized)
	request, err = http.NewRequestWithContext(c.context.context, method, uri, reader)
	if err != nil {
		c.logger.Error("Не удалось подготовить POST-запрос с {type}-данными: {error}", Logger.Context{
			"type":  data.GetType(),
			"error": err,
		})
		return nil
	}
	request.Header.Set("Content-Type", data.GetType())
	request.Header.Set("Content-Length", strconv.Itoa(reader.Len()))

	setHeaders(request, r.GetHeaders(), c.logger)
	setCookies(request, r.GetCookies(), c.logger)

	return c.send(request)
}

func (c *Client) send(request *http.Request) *Response {
	var response *http.Response
	var dump []byte
	var err error

	dump, err = httputil.DumpRequestOut(request, true)
	if err != nil {
		c.logger.Warn("Не удалось получить тело {method}-запроса: {error}", Logger.Context{
			"method": request.Method,
			"error":  err,
		})
	}

	var transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !c.context.secure,
		},
		DisableKeepAlives: true,
	}
	var client = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(c.context.timeout) * time.Second,
	}
	c.logger.Debug("Отправка {method}-запроса на адрес {uri}", Logger.Context{
		"method": request.Method,
		"uri":    request.URL.String(),
	})
	response, err = client.Do(request)
	if err != nil {
		c.logger.Error("Не удалось отправить {method}-запрос на адрес {uri}: {error}", Logger.Context{
			"method": request.Method,
			"uri":    request.URL.String(),
			"error":  err,
		})
		return nil
	}
	defer func() {
		err = response.Body.Close()
		if err != nil {
			c.logger.Error("Не удалось закрыть тело ответа: {error}", Logger.Context{
				"error": err,
			})
		}
	}()

	var r = c.parse(response)
	if r != nil {
		r.dump = dump
	}

	return r
}

func (c *Client) parse(response *http.Response) *Response {
	var code uint16
	var body []byte
	var headers = NewHeaders()
	var cookies = NewCookies()
	var err error

	body, err = io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error("Не удалось прочитать тело ответа: {error}", Logger.Context{
			"error": err,
		})
		return nil
	}

	code = uint16(response.StatusCode)
	for name := range maps.Keys(response.Header) {
		headers.Add(name, response.Header.Get(name))
	}
	for _, cookie := range response.Cookies() {
		cookies.Set(cookie.Name, cookie.Value)
	}

	return NewResponse(code, body, headers, cookies)
}

func getQuery(u string, query *Query, logger Logger.Logger) string {
	var parsed *url.URL
	var values url.Values
	var name, value string
	var err error

	logger.Debug("Добавление параметров в запрос: {count}", Logger.Context{
		"count": query.Size(),
	})
	parsed, err = url.Parse(u)
	if err != nil {
		logger.Error("Не удалось разобрать адрес HTTP-запроса '{uri}': {error}", Logger.Context{
			"uri":   u,
			"error": err,
		})
		return ""
	}

	values = parsed.Query()
	for name, value = range query.Iterate() {
		values.Set(name, value)
		logger.Debug(" - Добавлен параметр запроса: {name}", Logger.Context{
			"name": name,
		})
	}
	parsed.RawQuery = values.Encode()

	return parsed.String()
}

func setCookies(r *http.Request, cookies *Cookies, logger Logger.Logger) {
	logger.Debug("Добавление печенек в запрос: {count}", Logger.Context{
		"count": cookies.Size(),
	})
	for name, cookie := range cookies.Iterate() {
		r.AddCookie(cookie)
		logger.Debug(" - Добавлена печенька: {name}", Logger.Context{
			"name": name,
		})
	}
}

func setHeaders(r *http.Request, headers *Headers, logger Logger.Logger) {
	logger.Debug("Добавление заголовков в запрос: {count}", Logger.Context{
		"count": headers.Size(),
	})
	for name, value := range headers.Iterate() {
		r.Header.Set(name, value)
		logger.Debug(" - Добавлен заголовок: {name}", Logger.Context{
			"name": name,
		})
	}
}
