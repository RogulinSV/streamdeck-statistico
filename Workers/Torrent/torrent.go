package Torrent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
)

const (
	utorrentAPIPath = "/api.html"
	utorrentTimeout = 5 * time.Second
)

type Runner struct {
	logger Logger.Logger
}

func NewRunner(logger Logger.Logger) Scheduler.Runner {
	return &Runner{
		logger: logger,
	}
}

type utTorrent struct {
	Size         float64 `json:"size"`
	Progress     float64 `json:"progress"`
	State        int     `json:"state"`
	Name         string  `json:"name"`
	DownloadRate float64 `json:"download_rate"`
	UploadRate   float64 `json:"upload_rate"`
	Peers        int     `json:"peers"`
}

type utListResponse struct {
	Torrents []utTorrent `json:"torrents"`
}

func (r *Runner) Run(ctx context.Context, result chan<- Scheduler.Result) {
	if ctx.Err() != nil {
		return
	}

	// Получаем конфигурацию из переменных окружения или используем значения по умолчанию
	host := getenv("UTORRENT_HOST", "localhost:8080")
	username := getenv("UTORRENT_USERNAME", "admin")
	password := getenv("UTORRENT_PASSWORD", "admin")

	// Выполняем запрос к uTorrent API
	torrents, err := r.getTorrents(host, username, password)
	if err != nil {
		r.logger.Error("Не удалось получить список торрентов: {error}", Logger.Context{
			"error": err,
		})
		return
	}

	// Агрегируем данные по всем торрентам
	var totalDownloadRate float64
	var totalUploadRate float64
	var totalPeers int

	for _, torrent := range torrents {
		totalDownloadRate += torrent.DownloadRate
		totalUploadRate += torrent.UploadRate
		totalPeers += torrent.Peers
	}

	// Отправляем результаты
	if ctx.Err() == nil {
		r.logger.Debug("Отправка ответа", Logger.Context{})
		result <- Scheduler.AddMetric("clients", Scheduler.NewMetric(
			"Клиенты",
			strconv.Itoa(totalPeers),
		)).AddMetric("download_speed", Scheduler.NewMetric(
			"Скорость загрузки",
			formatSpeed(totalDownloadRate),
		)).AddMetric("upload_speed", Scheduler.NewMetric(
			"Скорость отдачи",
			formatSpeed(totalUploadRate),
		))
	}
}

func (r *Runner) getTorrents(host, username, password string) ([]utTorrent, error) {
	// Создаем HTTP клиент с таймаутом
	client := &http.Client{
		Timeout: utorrentTimeout,
	}

	// Формируем заголовок Authorization
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	// Сначала получаем token из заголовка ответа
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+host+utorrentAPIPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Считываем токен из заголовка X-Transmission-Session-Id или uTorrent token
	token := resp.Header.Get("X-Transmission-Session-Id")
	if token == "" {
		token = resp.Header.Get("uTorrentToken")
	}

	// Если токен не получен, пробуем прочитать тело ответа
	if token == "" {
		body, _ := io.ReadAll(resp.Body)
		r.logger.Debug("Ответ API без токена: {body}", Logger.Context{
			"body": string(body),
		})
	}

	// Формируем URL для запроса списка торрентов с токеном
	torrentsURL := fmt.Sprintf("http://%s/gui/?token=%s&action=list2", host, url.QueryEscape(token))

	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, torrentsURL, nil)
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Authorization", "Basic "+auth)

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	// Читаем и парсим ответ
	body, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, err
	}

	// Парсим JSON ответ
	var result utListResponse

	if err := json.Unmarshal(body, &result); err != nil {
		r.logger.Error("Не удалось распарсить ответ от uTorrent API: {error}", Logger.Context{
			"error": err,
			"body":  string(body),
		})
		return nil, fmt.Errorf("не удалось распарсить ответ от uTorrent API: %v", err)
	}

	return result.Torrents, nil
}

func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond >= 1024*1024 {
		return fmt.Sprintf("%.2f МБ/с", bytesPerSecond/(1024*1024))
	} else if bytesPerSecond >= 1024 {
		return fmt.Sprintf("%.2f КБ/с", bytesPerSecond/1024)
	}
	return fmt.Sprintf("%.0f Б/с", bytesPerSecond)
}

func getenv(key, defaultValue string) string {
	// Заглушка для получения переменных окружения
	// В реальной реализации можно использовать os.Getenv
	return defaultValue
}
