package WebSocket

import (
	"fmt"
	"net/http"

	"github.com/RogulinSV/streamdeck-statistico/v2/Http"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/gorilla/websocket"
)

// Message структура websocket-сообщения
type Message struct {
	code int
	data []byte
}

// NewMessage конструктор websocket-сообщения
func NewMessage(code int, data []byte) *Message {
	return &Message{
		code: code,
		data: data,
	}
}

// Describe метод реализует описание websocket-сообщения
func (m *Message) Describe() string {
	return fmt.Sprintf("тип %s длина %d байт", m.GetType(), m.GetSize())
}

// IsText метод реализует проверку текстового типа websocket-сообщения
func (m *Message) IsText() bool {
	return m.GetType() == "text"
}

// IsBinary метод реализует проверку двоичного типа websocket-сообщения
func (m *Message) IsBinary() bool {
	return m.GetType() == "binary"
}

// IsClose метод реализует проверку закрывающего типа websocket-сообщения
func (m *Message) IsClose() bool {
	return m.GetType() == "close"
}

// IsPing метод реализует проверку ping-типа websocket-сообщения
func (m *Message) IsPing() bool {
	return m.GetType() == "ping"
}

// IsPong метод реализует проверку pong-типа websocket-сообщения
func (m *Message) IsPong() bool {
	return m.GetType() == "pong"
}

// GetType метод реализует получение
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

func (m *Message) GetSize() int {
	return len(m.data)
}

type Connection struct {
	connection *websocket.Conn
	logger     Logger.Logger
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewConnection(stack Http.Stack, logger Logger.Logger) *Connection {
	var connection *websocket.Conn
	var err error

	logger.Debug("Открытие WS-соединения", Logger.Context{})
	connection, err = upgrader.Upgrade(stack.GetResponse(), stack.GetRequest(), nil)
	if err != nil {
		logger.Error("Не удалось открыть WS-соединение: {error}", Logger.Context{
			"error": err,
		})
		return nil
	}

	return &Connection{
		connection: connection,
		logger:     logger,
	}
}

func (c *Connection) Read() *Message {
	var message *Message
	var code int
	var data []byte
	var err error

	c.logger.Debug("Чтение WS-сообщения", Logger.Context{})
	code, data, err = c.connection.ReadMessage()
	if err != nil {
		c.logger.Error("Не удалось прочитать WS-сообщение: {error}", Logger.Context{
			"error": err,
		})
		return nil
	}

	message = NewMessage(code, data)
	c.logger.Debug("Получено WS-сообщение: {message}", Logger.Context{
		"message": message.Describe(),
	})

	return message
}

func (c *Connection) Write(message *Message) bool {
	var err error

	c.logger.Debug("Отправка WS-сообщения: {message}", Logger.Context{
		"message": message.Describe(),
	})
	err = c.connection.WriteMessage(message.code, message.data)
	if err != nil {
		c.logger.Error("Не удалось отправить WS-сообщение: {error}", Logger.Context{
			"error": err,
		})
		return false
	}

	return true
}

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
