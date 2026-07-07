package WebSocket

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/gorilla/websocket"
)

// Message структура вебсокетного сообщения
type Message struct {
	code int
	data []byte
}

// NewMessage конструктор вебсокетного сообщения
func NewMessage(code int, data []byte) *Message {
	return &Message{
		code: code,
		data: data,
	}
}

// NewTextMessage конструктор вебсокетного сообщения текстового формата
func NewTextMessage(data []byte) *Message {
	return &Message{
		code: websocket.TextMessage,
		data: data,
	}
}

// Describe метод реализует описание вебсокетного сообщения
func (m *Message) Describe() string {
	return fmt.Sprintf("тип %s длина %d байт", m.GetType(), m.GetSize())
}

// IsText метод реализует проверку текстового типа вебсокетного сообщения
func (m *Message) IsText() bool {
	return m.GetType() == "text"
}

// GetText метод реализует получение текстового сообщения
func (m *Message) GetText() string {
	return string(m.data)
}

// IsBinary метод реализует проверку двоичного типа вебсокетного сообщения
func (m *Message) IsBinary() bool {
	return m.GetType() == "binary"
}

// IsClose метод реализует проверку закрывающего типа вебсокетного сообщения
func (m *Message) IsClose() bool {
	return m.GetType() == "close"
}

// IsPing метод реализует проверку ping-типа вебсокетного сообщения
func (m *Message) IsPing() bool {
	return m.GetType() == "ping"
}

// IsPong метод реализует проверку pong-типа вебсокетного сообщения
func (m *Message) IsPong() bool {
	return m.GetType() == "pong"
}

// GetType метод реализует получение текстового описания статуса вебсокетного сообщения
func (m *Message) GetType() string {
	switch m.code {
	case websocket.TextMessage:
		return "text"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.CloseMessage:
		return "close"
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	default:
		return "unknown"
	}
}

// GetSize метод реализует получение длины вебсокетного сообщения
func (m *Message) GetSize() int {
	return len(m.data)
}

// Connection структура обработчика вебсокетного подключения
type Connection struct {
	connection   *websocket.Conn
	readTimeout  uint
	readLimit    uint32
	writeTimeout uint
	logger       Logger.Logger
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewConnection конструктор обработчика вебсокетного подключения
func NewConnection(response http.ResponseWriter, request *http.Request, logger Logger.Logger) *Connection {
	var connection *websocket.Conn
	var err error

	logger.Debug("Обновление HTTP-соединения до WS-соединения", Logger.Context{})
	connection, err = upgrader.Upgrade(response, request, nil)
	if err != nil {
		logger.Error("Не удалось обновить HTTP-соединение до WS-соединения: {error}", Logger.Context{
			"error": err,
		})
		return nil
	}

	return &Connection{
		connection:   connection,
		readTimeout:  60,
		readLimit:    1 << 20,
		writeTimeout: 10,
		logger:       logger,
	}
}

// Read метод реализует чтение входящих вебсокетных сообщений
func (c *Connection) Read() *Message {
	var message *Message
	var code int
	var data []byte
	var err error

	err = c.connection.SetReadDeadline(time.Now().Add(time.Duration(c.readTimeout) * time.Second))
	if err != nil {
		c.logger.Error("Не удалось установить таймаут чтения WS-сообщения {timeout} сек: {error}", Logger.Context{
			"timeout": c.readTimeout,
			"error":   err,
		})
	}
	c.connection.SetReadLimit(int64(c.readLimit))

	c.logger.Debug("Чтение WS-сообщения", Logger.Context{})
	code, data, err = c.connection.ReadMessage()
	if err != nil {
		if c.isBroken(err) {
			c.logger.Debug("Соединение закрыто клиентом", Logger.Context{})
		} else if c.isTimeout(err) {
			c.logger.Warn("Соединение закрыто сервером по таймауту", Logger.Context{})
		} else {
			c.logger.Error("Не удалось прочитать WS-сообщение: {error}", Logger.Context{
				"error": err,
			})
		}
		return nil
	}

	message = NewMessage(code, data)
	c.logger.Debug("Получено WS-сообщение: {message}", Logger.Context{
		"message": message.Describe(),
	})

	return message
}

// Write метод реализует запись исходящих вебсокетных сообщений
func (c *Connection) Write(message *Message) bool {
	var err error

	err = c.connection.SetWriteDeadline(time.Now().Add(time.Duration(c.writeTimeout) * time.Second))
	if err != nil {
		c.logger.Error("Не удалось установить таймаут отправки WS-сообщения {timeout} сек: {error}", Logger.Context{
			"timeout": c.writeTimeout,
			"error":   err,
		})
	}

	c.logger.Debug("Отправка WS-сообщения: {message}", Logger.Context{
		"message": message.Describe(),
	})
	err = c.connection.WriteMessage(message.code, message.data)
	if err != nil {
		if c.isBroken(err) {
			c.logger.Debug("Соединение закрыто клиентом", Logger.Context{})
		} else if c.isTimeout(err) {
			c.logger.Warn("Соединение закрыто сервером по таймауту", Logger.Context{})
		} else {
			c.logger.Error("Не удалось отправить WS-сообщение: {error}", Logger.Context{
				"error": err,
			})
		}
		return false
	}

	return true
}

// Close метод реализует закрытие вебсокетного подключения
func (c *Connection) Close() bool {
	var err error

	c.logger.Debug("Закрытие WS-соединения", Logger.Context{})
	err = c.connection.Close()
	if err != nil {
		c.logger.Error("Не удалось закрыть WS-соединение: {error}", Logger.Context{
			"error": err,
		})
		return false
	}

	return true
}

func (c *Connection) isBroken(err error) bool {
	if err == nil {
		return false
	}

	// Соединение корректно внешней стороной
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
		return true
	}

	// Соединение обрывается без close-фрейма
	if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "connection reset by peer") || strings.Contains(err.Error(), "broken pipe") {
		return true
	}

	return false
}

func (c *Connection) isTimeout(err error) bool {
	var netErr net.Error

	if err != nil {
		return errors.As(err, &netErr) && netErr.Timeout()
	}

	return false
}
