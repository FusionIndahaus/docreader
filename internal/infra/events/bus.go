package events

import (
	"encoding/json"
	"strings"
	"sync"

	domain "document-ai/internal/domain"
)

type Bus struct {
	responses      *[]domain.ProcessingResponse
	responsesMutex *sync.RWMutex
	subscribers    *map[chan domain.ProcessingResponse]struct{}
	subscribersMux *sync.RWMutex
}

func NewBus(
	responses *[]domain.ProcessingResponse,
	responsesMutex *sync.RWMutex,
	subscribers *map[chan domain.ProcessingResponse]struct{},
	subscribersMux *sync.RWMutex,
) Bus {
	return Bus{
		responses:      responses,
		responsesMutex: responsesMutex,
		subscribers:    subscribers,
		subscribersMux: subscribersMux,
	}
}

func (b Bus) Subscribe(userEmail string) (<-chan []byte, func()) {
	out := make(chan []byte, 1)
	ch := make(chan domain.ProcessingResponse, 1)
	// register
	b.subscribersMux.Lock()
	if *b.subscribers == nil {
		*b.subscribers = make(map[chan domain.ProcessingResponse]struct{})
	}
	(*b.subscribers)[ch] = struct{}{}
	b.subscribersMux.Unlock()
	// adapter goroutine
	go func(filter string) {
		for resp := range ch {
			if filter != "" && resp.UserEmail != "" && !strings.EqualFold(resp.UserEmail, filter) {
				continue
			}
			if bts, err := json.Marshal(resp); err == nil {
				select {
				case out <- bts:
				default:
				}
			}
		}
		close(out)
	}(strings.ToLower(userEmail))
	unsubscribe := func() {
		b.subscribersMux.Lock()
		delete(*b.subscribers, ch)
		close(ch)
		b.subscribersMux.Unlock()
	}
	return out, unsubscribe
}

func (b Bus) InitialSnapshot(email string) [][]byte {
	b.responsesMutex.RLock()
	defer b.responsesMutex.RUnlock()
	var out [][]byte
	lc := strings.ToLower(email)
	for _, resp := range *b.responses {
		if resp.UserEmail != "" && strings.EqualFold(resp.UserEmail, lc) {
			if bs, err := json.Marshal(resp); err == nil {
				out = append(out, bs)
			}
		}
	}
	return out
}

// Publish добавляет событие и оповещает подписчиков
func (b Bus) Publish(resp domain.ProcessingResponse) {
	b.responsesMutex.Lock()
	*b.responses = append(*b.responses, resp)
	if len(*b.responses) > 0 {
		// Ограничение списка ответов, если глобальный лимит задан вовне — предполагаем, что внешняя логика обрежет.
	}
	b.responsesMutex.Unlock()
	// рассылка подписчикам
	b.subscribersMux.RLock()
	for ch := range *b.subscribers {
		select {
		case ch <- resp:
		default:
		}
	}
	b.subscribersMux.RUnlock()
}
