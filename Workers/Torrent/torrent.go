package Torrent

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Context"
	"github.com/RogulinSV/streamdeck-statistico/v2/Format"
	"github.com/RogulinSV/streamdeck-statistico/v2/Http"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
)

// see https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-5.0)
const (
	loginUrl         = "/api/v2/auth/login"
	logoutUrl        = "/api/v2/auth/logout"
	clientVersionUrl = "/api/v2/app/version"
	transferInfoUrl  = "/api/v2/transfer/info"
	torrentsInfoUrl  = "/api/v2/torrents/info"
)

type Session struct {
	name   string
	token  string
	expire time.Time
}

func NewSession(name string, token string, expire time.Time) *Session {
	return &Session{
		name:   name,
		token:  token,
		expire: expire,
	}
}

func (s *Session) Expired() bool {
	return !s.expire.IsZero() && s.expire.Before(time.Now())
}

func (s *Session) bind(request Http.Request) {
	request.GetCookies().Set(s.name, s.token)
}

type Config struct {
	schema   string
	host     string
	port     string
	username string
	password string
	timeout  uint
	session  *Session
	logout   bool
	filter   Filter
}

func NewConfig(uri string, username string, password string) *Config {
	var parsed, err = url.Parse(uri)
	var schema = "http"
	var host = "localhost"
	var port = "8080"
	if err == nil {
		schema = parsed.Scheme
		host = parsed.Hostname()
		port = parsed.Port()
	}

	return &Config{
		schema:   schema,
		host:     host,
		port:     port,
		username: username,
		password: password,
		timeout:  5,
		logout:   false,
		filter: Filter{
			State: TorrentStateActive,
		},
	}
}

func (c *Config) SetTimeout(timeout uint) *Config {
	c.timeout = timeout

	return c
}

func (c *Config) ForceLogout() *Config {
	c.logout = true

	return c
}

func (c *Config) SetFilter(filter Filter) *Config {
	c.filter = filter

	return c
}

func (c *Config) GetUri() string {
	return fmt.Sprintf("%s://%s:%s", c.schema, c.host, c.port)
}

type transferInfo struct {
	DownloadRate    int `json:"dl_info_speed"`
	DownloadedBytes int `json:"dl_info_data"`
	UploadRate      int `json:"up_info_speed"`
	UploadedBytes   int `json:"up_info_data"`
	Nodes           int `json:"dht_nodes"`
}

func (i *transferInfo) describe() string {
	return fmt.Sprintf("Download Rate: %s, Upload Rate: %s, Downloaded Bytes: %s, Uploaded Bytes: %s",
		Format.ToRate(i.DownloadRate),
		Format.ToRate(i.UploadRate),
		Format.ToByte(i.DownloadedBytes),
		Format.ToByte(i.UploadedBytes),
	)
}

type torrentInfo struct {
	Size       int    `json:"size"`
	AmountLeft int    `json:"amount_left"`
	Uploaded   int    `json:"uploaded"`
	State      string `json:"state"`
	Seeds      int    `json:"num_seeds"`
	Leechers   int    `json:"num_leechs"`
}

type torrentsList []*torrentInfo

func (l *torrentsList) describe() string {
	return fmt.Sprintf("Torrents: %d", len(*l))
}

type FilterByState string

const (
	TorrentStateAll                FilterByState = "all"
	TorrentStateDownloading        FilterByState = "downloading"
	TorrentStateSeeding            FilterByState = "seeding"
	TorrentStateCompleted          FilterByState = "completed"
	TorrentStateStopped            FilterByState = "stopped"
	TorrentStateActive             FilterByState = "active"
	TorrentStateInactive           FilterByState = "inactive"
	TorrentStateRunning            FilterByState = "running"
	TorrentStateStalled            FilterByState = "stalled"
	TorrentStateStalledUploading   FilterByState = "stalled_uploading"
	TorrentStateStalledDownloading FilterByState = "stalled_downloading"
	TorrentStateErrored            FilterByState = "errored"
)

// FilterByCategory parameter get torrents with the given category
// Empty string means "without category"
// No "category" parameter (nil) means "any category"
type FilterByCategory string

// FilterByTag parameter get torrents with the given tag
// Empty string means "without tag"
// No "tag" parameter (nil) means "any tag"
type FilterByTag string

type Filter struct {
	State    FilterByState
	Category *FilterByCategory
	Tag      *FilterByTag
	Sort     string
	Reverse  bool
	Limit    uint
	Offset   uint
}

func NewFilter(state FilterByState, category *FilterByCategory, tag *FilterByTag) Filter {
	return Filter{
		State:    state,
		Category: category,
		Tag:      tag,
	}
}

type aggregateStatistics struct {
	total       int
	errors      int
	uploading   int
	uploaded    int
	downloading int
	seeds       int
	leechers    int
	amountLeft  int
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

func (r *Runner) Defaults() Scheduler.Result {
	return Scheduler.AddMetric(
		"dl_rate",
		Scheduler.NewMetric("Загрузка", "0").WithUnits(Format.DefaultRateUnit),
	).AddMetric(
		"dl_bytes",
		Scheduler.NewMetric("Загружено", "0").WithUnits(Format.DefaultByteUnit),
	).AddMetric(
		"up_rate",
		Scheduler.NewMetric("Отдача", "0").WithUnits(Format.DefaultRateUnit),
	).AddMetric(
		"up_bytes",
		Scheduler.NewMetric("Отдано", "0").WithUnits(Format.DefaultByteUnit),
	).AddMetric(
		"torrents_total",
		Scheduler.NewMetric("Торренты", "0"),
	).AddMetric(
		"torrents_errors",
		Scheduler.NewMetric("Ошибки", "0"),
	).AddMetric(
		"torrents_uploading",
		Scheduler.NewMetric("Раздаётся", "0"),
	).AddMetric(
		"bytes_uploaded",
		Scheduler.NewMetric("Отдано", "0").WithUnits(Format.DefaultByteUnit),
	).AddMetric(
		"torrents_downloading",
		Scheduler.NewMetric("Загружается", "0"),
	).AddMetric(
		"torrents_seeds",
		Scheduler.NewMetric("Сиды", "0"),
	).AddMetric(
		"torrents_leechers",
		Scheduler.NewMetric("Личи", "0"),
	).AddMetric(
		"torrents_left",
		Scheduler.NewMetric("Остаток", "0").WithUnits(Format.DefaultByteUnit),
	)
}

func (r *Runner) Run(context context.Context, result chan<- Scheduler.Result) {
	var http = Http.NewClient(
		Http.NewContext(r.config.timeout, context),
		r.logger,
	)

	var uri = r.config.GetUri()
	if r.config.session == nil {
		r.config.session = r.login(http, uri, r.config.username, r.config.password)
		if r.config.session != nil {
			r.getClientVersion(http, r.config.session, uri)
		}
	}
	if r.config.session != nil {
		defer func() {
			if r.config.session.Expired() || r.config.logout {
				r.logout(http, r.config.session, uri)
				r.config.session = nil
			}
		}()

		var info = r.getTransferInfo(http, r.config.session, uri)
		if info != nil {
			var rate *Format.RateUnit
			var bytes *Format.ByteUnit

			if !Context.Canceled(context) {
				rate = Format.ToRate(info.DownloadRate)
				bytes = Format.ToByte(info.DownloadedBytes)
				result <- Scheduler.AddMetric(
					"dl_rate",
					Scheduler.NewMetric("Загрузка", strconv.Itoa(rate.Value)).WithUnits(rate.Units),
				).AddMetric(
					"dl_bytes",
					Scheduler.NewMetric("Загружено", strconv.Itoa(bytes.Value)).WithUnits(bytes.Units),
				)
			}

			if !Context.Canceled(context) {
				rate = Format.ToRate(info.UploadRate)
				bytes = Format.ToByte(info.UploadedBytes)
				result <- Scheduler.AddMetric(
					"up_rate",
					Scheduler.NewMetric("Отдача", strconv.Itoa(rate.Value)).WithUnits(rate.Units),
				).AddMetric(
					"up_bytes",
					Scheduler.NewMetric("Отдано", strconv.Itoa(bytes.Value)).WithUnits(bytes.Units),
				)
			}
		}

		var torrents = r.getTorrentsList(http, r.config.session, uri, r.config.filter)
		if torrents != nil {
			var aggregate = aggregateStatistics{
				total: len(*torrents),
			}
			for _, torrent := range *torrents {
				switch torrent.State {
				case "error":
					aggregate.errors++
				case "uploading":
					aggregate.uploading++
				case "downloading":
					aggregate.downloading++
				}
				aggregate.seeds += torrent.Seeds
				aggregate.leechers += torrent.Leechers
				aggregate.amountLeft += torrent.AmountLeft
				aggregate.uploaded += torrent.Uploaded
			}

			if !Context.Canceled(context) {
				var bytesLeft = Format.ToByte(aggregate.amountLeft)
				var bytesUploaded = Format.ToByte(aggregate.uploaded)

				result <- Scheduler.AddMetric(
					"torrents_total",
					Scheduler.NewMetric("Торренты", strconv.Itoa(aggregate.total)),
				).AddMetric(
					"torrents_errors",
					Scheduler.NewMetric("Ошибки", strconv.Itoa(aggregate.errors)),
				).AddMetric(
					"torrents_uploading",
					Scheduler.NewMetric("Раздаётся", strconv.Itoa(aggregate.uploading)),
				).AddMetric(
					"bytes_uploaded",
					Scheduler.NewMetric("Отдано", strconv.Itoa(bytesUploaded.Value)).WithUnits(bytesUploaded.Units),
				).AddMetric(
					"torrents_downloading",
					Scheduler.NewMetric("Загружается", strconv.Itoa(aggregate.downloading)),
				).AddMetric(
					"torrents_seeds",
					Scheduler.NewMetric("Сиды", strconv.Itoa(aggregate.seeds)),
				).AddMetric(
					"torrents_leechers",
					Scheduler.NewMetric("Личи", strconv.Itoa(aggregate.leechers)),
				).AddMetric(
					"torrents_left",
					Scheduler.NewMetric("Остаток", strconv.Itoa(bytesLeft.Value)).WithUnits(bytesLeft.Units),
				)
			}
		}
	}
}

func (r *Runner) login(http *Http.Client, uri string, username string, password string) *Session {
	var name = "QBT_SID_" + r.config.port
	var value string
	var expires = time.Now()

	var data = Http.NewFormData()
	data.Set("username", username)
	data.Set("password", password)

	var request = Http.NewPostRequest(uri+loginUrl, data)
	Http.SetReferrerHeader(request, uri)
	Http.SetUserAgentHeader(request, Http.DefaultUserAgent)

	r.logger.Debug("Получение сессии клиента", Logger.Context{})
	var response = http.Post(request)
	if response != nil {
		if response.IsSuccessful() {
			var cookie = response.GetCookies().Get(name)
			if cookie != nil {
				value = cookie.Value
				expires = cookie.Expires
				r.logger.Info("Получена сессия клиента: {expires}", Logger.Context{
					"expires": expires,
				})
			} else {
				r.logger.Error("Отсутствует сессия клиента", Logger.Context{})
			}
		} else {
			switch true {
			case response.IsUnauthorized():
				r.logger.Error("Не удалось получить сессию клиента: некорректные данные", Logger.Context{})
			case response.IsForbidden():
				r.logger.Error("Не удалось получить сессию клиента: доступ запрещён", Logger.Context{})
			default:
				r.logger.Error("Не удалось получить сессию клиента: код ответа {code}", Logger.Context{
					"code": response.GetCode(),
				})
			}
		}
	} else {
		r.logger.Error("Не удалось получить сессию клиента", Logger.Context{})
	}

	if value == "" {
		return nil
	}

	return NewSession(name, value, expires)
}

func (r *Runner) logout(http *Http.Client, session *Session, uri string) bool {
	var closed = false

	var request = Http.NewPostRequest(uri+logoutUrl, nil)
	session.bind(request)
	Http.SetReferrerHeader(request, uri)
	Http.SetUserAgentHeader(request, Http.DefaultUserAgent)

	r.logger.Debug("Завершение сессии клиента", Logger.Context{})
	var response = http.Post(request)
	if response != nil {
		if response.IsSuccessful() {
			r.logger.Info("Закрыта сессия клиента", Logger.Context{})
			closed = true
		} else {
			r.logger.Error("Не удалось закрыть сессию клиента: код ответа {code}", Logger.Context{
				"code": response.GetCode(),
			})
		}
	} else {
		r.logger.Error("Не удалось закрыть сессию клиента", Logger.Context{})
	}

	return closed
}

func (r *Runner) getClientVersion(http *Http.Client, session *Session, uri string) string {
	var version string

	var request = Http.NewPostRequest(uri+clientVersionUrl, nil)
	session.bind(request)
	Http.SetReferrerHeader(request, uri)
	Http.SetUserAgentHeader(request, Http.DefaultUserAgent)

	r.logger.Debug("Получение версии клиента", Logger.Context{})
	var response = http.Post(request)
	if response != nil {
		if response.IsSuccessful() {
			version = response.GetBody()
			r.logger.Info("Получена версия клиента: {version}", Logger.Context{
				"version": version,
			})
		} else {
			r.logger.Error("Не удалось получить версию клиента: код ответа {code}", Logger.Context{
				"code": response.GetCode(),
			})
		}
	} else {
		r.logger.Error("Не удалось получить версию клиента", Logger.Context{})
	}

	return version
}

func (r *Runner) getTransferInfo(http *Http.Client, session *Session, uri string) *transferInfo {
	var info *transferInfo
	var err error

	var request = Http.NewPostRequest(uri+transferInfoUrl, nil)
	session.bind(request)
	Http.SetAcceptJsonHeader(request)
	Http.SetReferrerHeader(request, uri)
	Http.SetUserAgentHeader(request, Http.DefaultUserAgent)

	r.logger.Debug("Получение статистики по сетевой активности", Logger.Context{})
	var response = http.Post(request)
	if response != nil {
		if response.IsSuccessful() {
			if response.IsJson() {
				info = new(transferInfo)
				err = response.ToJson(info)
				if err != nil {
					r.logger.Error("Не удалось прочитать статистику по сетевой активности: {error}", Logger.Context{
						"error": err,
					})
				} else {
					r.logger.Info("Получена статистика по сетевой активности: {info}", Logger.Context{
						"info": info.describe(),
					})
				}
			} else {
				r.logger.Error("Не удалось получить статистику по сетевой активности: тип ответа {type}", Logger.Context{
					"type": response.GetType(),
				})
			}
		} else {
			r.logger.Error("Не удалось получить статистику по сетевой активности: код ответа {code}", Logger.Context{
				"code": response.GetCode(),
			})
		}
	} else {
		r.logger.Error("Не удалось получить статистику по сетевой активности", Logger.Context{})
	}

	return info
}

func (r *Runner) getTorrentsList(http *Http.Client, session *Session, uri string, filter Filter) *torrentsList {
	var torrents *torrentsList
	var err error

	var request = Http.NewPostRequest(uri+torrentsInfoUrl, nil)
	var query = request.GetQuery()
	query.Set("filter", string(filter.State))
	if filter.Category != nil {
		query.Set("category", string(*filter.Category))
	}
	if filter.Tag != nil {
		query.Set("tag", string(*filter.Tag))
	}
	if filter.Limit > 0 {
		query.Set("limit", strconv.Itoa(int(filter.Limit)))
	}
	if filter.Offset > 0 {
		query.Set("offset", strconv.Itoa(int(filter.Offset)))
	}
	if filter.Sort != "" {
		query.Set("sort", filter.Sort)
	}
	if filter.Reverse {
		query.Set("reverse", "true")
	}
	session.bind(request)
	Http.SetAcceptJsonHeader(request)
	Http.SetReferrerHeader(request, uri)
	Http.SetUserAgentHeader(request, Http.DefaultUserAgent)

	r.logger.Debug("Получение перечня торрентов", Logger.Context{})
	var response = http.Post(request)
	if response != nil {
		if response.IsSuccessful() {
			if response.IsJson() {
				torrents = new(torrentsList)
				err = response.ToJson(torrents)
				if err != nil {
					r.logger.Error("Не удалось прочитать перечень торрентов: {error}", Logger.Context{
						"error": err,
					})
				} else {
					r.logger.Info("Получен перечень торрентов: {torrents}", Logger.Context{
						"torrents": torrents.describe(),
					})
				}
			} else {
				r.logger.Error("Не удалось получить перечень торрентов: тип ответа {type}", Logger.Context{
					"type": response.GetType(),
				})
			}
		} else {
			r.logger.Error("Не удалось получить перечень торрентов: код ответа {code}", Logger.Context{
				"code": response.GetCode(),
			})
		}
	} else {
		r.logger.Error("Не удалось получить перечень торрентов", Logger.Context{})
	}

	return torrents
}
