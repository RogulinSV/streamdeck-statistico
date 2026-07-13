package Torrent

import (
	"context"
	"fmt"

	"github.com/RogulinSV/streamdeck-statistico/v2/Context"
	"github.com/RogulinSV/streamdeck-statistico/v2/Http"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 6.2; WOW64) AppleWebKit/537.46 (KHTML, like Gecko) Chrome/50.0.3922.369 Safari/600"
	authUrl   = "/api/v2/auth/login"
)

type Config struct {
	uri      string
	login    string
	password string
	timeout  uint
	token    string
}

func NewConfig(uri string, login string, password string) *Config {
	return &Config{
		uri:      uri,
		login:    login,
		password: password,
		timeout:  5,
	}
}

func (c *Config) SetTimeout(timeout uint) *Config {
	c.timeout = timeout

	return c
}

type Runner struct {
	config *Config
	logger Logger.Logger
}

func NewRunner(config *Config, logger Logger.Logger) Scheduler.Runner {
	return &Runner{
		config: config,
		logger: logger,
	}
}

func (r *Runner) Run(context context.Context, result chan<- Scheduler.Result) {
	var http = Http.NewClient(
		Http.NewContext(r.config.timeout, context),
		r.logger,
	)

	if r.config.token == "" {
		r.config.token = r.getToken(http)
		if Context.Canceled(context) {
			return
		}
	}
}

func (r *Runner) getToken(http *Http.Client) string {
	var token string

	r.logger.Debug("Получение сессии клиента", Logger.Context{})

	var uri = r.config.uri + authUrl
	var data = Http.NewFormData(nil).Set("login", r.config.login).Set("password", r.config.password)
	var request = Http.NewPostFormRequest(uri, data, nil)
	Http.SetReferrerHeader(request, r.config.uri)
	Http.SetUserAgentHeader(request, userAgent)

	var response = http.PostForm(request)
	if response != nil {
		if response.GetHeaders().Has("X-Transmission-Session-Id") {
			token = response.GetHeaders().Get("X-Transmission-Session-Id")
			r.logger.Info("{token}", Logger.Context{
				"token": token,
			})
		}
	} else {
		r.logger.Error("Не удалось получить сессию клиента", Logger.Context{})
	}

	return token
}

func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond >= 1024*1024 {
		return fmt.Sprintf("%.2f МБ/с", bytesPerSecond/(1024*1024))
	} else if bytesPerSecond >= 1024 {
		return fmt.Sprintf("%.2f КБ/с", bytesPerSecond/1024)
	}
	return fmt.Sprintf("%.0f Б/с", bytesPerSecond)
}
