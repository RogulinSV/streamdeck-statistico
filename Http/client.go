package Http

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
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
	var buffer = &bytes.Buffer{}
	var request *http.Request
	var uri string
	var err error

	uri = getQuery(r.url, r.query, c.logger)
	if uri == "" {
		return nil
	}

	request, err = http.NewRequestWithContext(c.context.context, method, uri, &progress{
		Reader: buffer,
		total:  buffer.Len(),
		progress: func(percent uint8) {
			c.context.progress(percent, c.logger)
		},
	})
	if err != nil {
		c.logger.Error("", Logger.Context{
			"error": err,
		})
		return nil
	}

	setHeaders(request, r.GetHeaders(), c.logger)
	setCookies(request, r.GetCookies(), c.logger)

	return c.send(request)
}

func (c *Client) PostForm(r *PostFormRequest) *Response {
	var method = http.MethodPost
	var buffer = &bytes.Buffer{}
	var writer = multipart.NewWriter(buffer)
	var closed = false
	var request *http.Request
	var uri string
	var err error

	var done = func() {
		if !closed {
			err = writer.Close()
			if err != nil {
				c.logger.Error("", Logger.Context{
					"error": err,
				})
			} else {
				closed = true
			}
		}
	}
	defer done()

	uri = getQuery(r.url, r.query, c.logger)
	if uri == "" {
		return nil
	}

	if r.HasFiles() {
		if !setFiles(writer, r.GetFiles(), c.logger) {
			return nil
		}
	}
	if r.HasData() {
		if !setData(writer, r.GetData(), c.logger) {
			return nil
		}
	}
	done()

	request, err = http.NewRequestWithContext(c.context.context, method, uri, &progress{
		Reader: buffer,
		total:  buffer.Len(),
		progress: func(percent uint8) {
			c.context.progress(percent, c.logger)
		},
	})
	if err != nil {
		c.logger.Error("", Logger.Context{
			"error": err,
		})
		return nil
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Content-Length", strconv.Itoa(buffer.Len()))

	setHeaders(request, r.GetHeaders(), c.logger)
	setCookies(request, r.GetCookies(), c.logger)

	return c.send(request)
}

func (c *Client) PostJson(r *PostJsonRequest) *Response {
	var method = http.MethodPost
	var reader *strings.Reader
	var request *http.Request
	var json string
	var uri string
	var err error

	uri = getQuery(r.url, r.query, c.logger)
	if uri == "" {
		return nil
	}

	json, err = r.GetJson().Serialize()
	if err != nil {
		c.logger.Error("", Logger.Context{})
		return nil
	}
	reader = strings.NewReader(json)

	request, err = http.NewRequestWithContext(c.context.context, method, uri, &progress{
		Reader: reader,
		total:  reader.Len(),
		progress: func(percent uint8) {
			c.context.progress(percent, c.logger)
		},
	})
	if err != nil {
		c.logger.Error("", Logger.Context{
			"error": err,
		})
		return nil
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Length", strconv.Itoa(reader.Len()))

	setHeaders(request, r.GetHeaders(), c.logger)
	setCookies(request, r.GetCookies(), c.logger)

	return c.send(request)
}

func (c *Client) send(request *http.Request) *Response {
	var response *http.Response
	var err error

	var transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !c.context.secure,
		},
	}
	var client = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(c.context.timeout) * time.Second,
	}
	response, err = client.Do(request)
	if err != nil {
		c.logger.Error("", Logger.Context{
			"error": err,
		})
		return nil
	}
	defer func() {
		err = response.Body.Close()
		if err != nil {
			c.logger.Error("", Logger.Context{
				"error": err,
			})
		}
	}()

	return c.parse(response)
}

func (c *Client) parse(response *http.Response) *Response {
	var code uint8
	var body []byte
	var headers = &Headers{}
	var cookies = &Cookies{}
	var err error

	body, err = io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error("", Logger.Context{
			"error": err,
		})
		return nil
	}

	code = uint8(response.StatusCode)
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
		logger.Error("Не удалось разобрать адрес HTTP-запроса: {error}", Logger.Context{
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

func setFiles(writer *multipart.Writer, f *Files, logger Logger.Logger) bool {
	var name, path string
	var err error

	logger.Debug("Добавление файлов в запрос: {count}", Logger.Context{
		"count": f.Size(),
	})

	var files []*os.File
	defer func() {
		for _, file := range files {
			err = file.Close()
			if err != nil {
				logger.Error("Не удалось закрыть файл '{path}': {error}", Logger.Context{
					"path":  file.Name(),
					"error": err,
				})
			}
		}
	}()

	for name, path = range f.Iterate() {
		var temp io.Writer
		var file *os.File
		var size int64
		temp, err = writer.CreateFormFile(name, path)
		if err != nil {
			logger.Error("Не удалось добавить файл '{path}' в HTTP-запрос: {error}", Logger.Context{
				"path":  path,
				"error": err,
			})
			return false
		}
		file, err = os.Open(path)
		if err != nil {
			logger.Error("Не удалось открыть файл '{path}' на чтение: {error}", Logger.Context{
				"path":  path,
				"error": err,
			})
			return false
		}
		files = append(files, file)
		size, err = io.Copy(temp, file)
		if err != nil {
			logger.Error("Не удалось переместить файл '{path}': {error}", Logger.Context{
				"path":  path,
				"error": err,
			})
			return false
		}
		logger.Debug(" - Добавлен файл: {name} {size} байт", Logger.Context{
			"name": name,
			"size": size,
		})
	}

	return true
}

func setData(writer *multipart.Writer, data *FormData, logger Logger.Logger) bool {
	var name, value string
	var err error

	logger.Debug("Добавление полей в запрос: {count}", Logger.Context{
		"count": data.Size(),
	})
	for name, value = range data.Iterate() {
		err = writer.WriteField(name, value)
		if err != nil {
			logger.Error("Не удалось добавить поле '{field}' в HTTP-запрос: {error}", Logger.Context{
				"field": name,
				"error": err,
			})
			return false
		}
		logger.Debug(" - Добавлено поле: {name}", Logger.Context{
			"name": name,
		})
	}

	return true
}
