package manager

import "sync"

type Event struct {
	Type     string `json:"type"`
	Instance string `json:"instance,omitempty"`
	Message  string `json:"message,omitempty"`
}

type Hub struct {
	mu   sync.Mutex
	subs map[int]chan Event
	next int
}

func NewHub() *Hub {
	return &Hub{subs: map[int]chan Event{}}
}

func (h *Hub) Subscribe() (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := h.next
	h.next++
	ch := make(chan Event, 32)
	h.subs[id] = ch

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if c, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(c)
		}
	}
}

func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}
