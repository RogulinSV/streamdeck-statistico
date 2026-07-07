package Workers

import (
	"strconv"
	"sync"
)

type Counter struct {
	mutex    *sync.Mutex
	value    int
	unsigned bool
}

func NewCounter(value int) *Counter {
	return &Counter{
		mutex:    &sync.Mutex{},
		value:    value,
		unsigned: true,
	}
}

func (c *Counter) Value() int {
	// c.mutex.Lock()
	// defer c.mutex.Unlock()

	return c.value
}

func (c *Counter) ToStr() string {
	return strconv.Itoa(c.Value())
}

func (c *Counter) Increment(value int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.value += value
}

func (c *Counter) Decrement(value int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.unsigned || c.value > value {
		c.value -= value
	} else {
		c.value = 0
	}
}
