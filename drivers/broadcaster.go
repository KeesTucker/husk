package drivers

import (
	"sync"

	"husk/canbus"
	"husk/logging"
	"husk/services"
)

type Broadcaster struct {
	subscribers map[chan *canbus.CanFrame]struct{}
	mutex       sync.Mutex
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
	b.mutex.Lock()
	b.subscribers[ch] = struct{}{}
	b.mutex.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
func (b *Broadcaster) Unsubscribe(ch chan *canbus.CanFrame) {
	b.mutex.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mutex.Unlock()
}

// Broadcast sends a frame to all subscribers.
func (b *Broadcaster) Broadcast(frame *canbus.CanFrame) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	b.mutex.Lock()
	for ch := range b.subscribers {
		// Non-blocking send to prevent goroutine leak if a subscriber is slow or not reading.
		select {
		case ch <- frame:
		default:
			l.WriteToLog("slow subscriber, frame channel is full.")
		}
	}
	b.mutex.Unlock()
}
