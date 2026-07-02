package WebSocket

import (
	"fmt"
	"net/http"

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
	connection *websocket.Conn
	logger     Logger.Logger
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
		connection: connection,
		logger:     logger,
	}
}

// Read метод реализует чтение входящих вебсокетных сообщений
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

// Write метод реализует запись исходящих вебсокетных сообщений
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
