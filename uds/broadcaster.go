package uds

import (
	"sync"

	"husk/logging"
	"husk/services"
)

// MessageBroadcaster broadcasts UDS messages received
type MessageBroadcaster struct {
	subscribers map[chan *Message]struct{}
	lock        sync.RWMutex
}

// NewUDSMessageBroadcaster creates a new UDSMessageBroadcaster.
func NewUDSMessageBroadcaster() *MessageBroadcaster {
	return &MessageBroadcaster{
		subscribers: make(map[chan *Message]struct{}),
	}
}

// Subscribe adds a new subscriber and returns a channel to receive messages from the UDS.
func (b *MessageBroadcaster) Subscribe() chan *Message {
	ch := make(chan *Message, 128)
	b.lock.Lock()
	b.subscribers[ch] = struct{}{}
	b.lock.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
func (b *MessageBroadcaster) Unsubscribe(ch chan *Message) {
	b.lock.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.lock.Unlock()
}

// Broadcast sends a message to all subscribers.
func (b *MessageBroadcaster) Broadcast(message *Message) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	b.lock.RLock()
	defer b.lock.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- message:
		default:
			l.WriteToLog("Error: slow subscriber, message channel is full. dropping message.")
		}
	}
}
func (b *MessageBroadcaster) Cleanup() {
	b.lock.Lock()
	for channel := range b.subscribers {
		delete(b.subscribers, channel)
		close(channel)
	}
	b.lock.Unlock()
}
