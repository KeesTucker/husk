package drivers

import (
	"sync"

	"husk/canbus"
	"husk/logging"
	"husk/services"
)

type Broadcaster struct {
	subscribers map[chan *canbus.CanFrame]struct{}
	lock        sync.RWMutex
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan *canbus.CanFrame]struct{}),
	}
}

// Subscribe adds a new subscriber and returns a channel to receive frames.
func (b *Broadcaster) Subscribe() chan *canbus.CanFrame {
	ch := make(chan *canbus.CanFrame, 128)
	b.lock.Lock()
	b.subscribers[ch] = struct{}{}
	b.lock.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
func (b *Broadcaster) Unsubscribe(ch chan *canbus.CanFrame) {
	b.lock.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.lock.Unlock()
}

// Broadcast sends a frame to all subscribers.
func (b *Broadcaster) Broadcast(frame *canbus.CanFrame) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	b.lock.RLock()
	defer b.lock.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- frame:
		default:
			l.WriteToLog("Error: slow subscriber, frame channel is full. dropping frame.")
		}
	}
}

func (b *Broadcaster) Cleanup() {
	b.lock.Lock()
	for channel := range b.subscribers {
		delete(b.subscribers, channel)
		close(channel)
	}
	b.lock.Unlock()
}
