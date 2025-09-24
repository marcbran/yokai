package run

import (
	"sync"
)

type Broker interface {
	Subscribe(topic Topic) (<-chan Payload, Unsubscribe)
	SubscribeAll() (<-chan TopicPayload, Unsubscribe)
	Publish(topic Topic, payload Payload)
}

type Unsubscribe func()

type MutexBroker struct {
	mu      sync.RWMutex
	topics  map[Topic]map[chan Payload]struct{}
	allSubs map[chan TopicPayload]struct{}
}

func NewBroker() *MutexBroker {
	return &MutexBroker{
		topics:  make(map[Topic]map[chan Payload]struct{}),
		allSubs: make(map[chan TopicPayload]struct{}),
	}
}

func (b *MutexBroker) Subscribe(topic Topic) (<-chan Payload, Unsubscribe) {
	ch := make(chan Payload, 16)

	b.mu.Lock()
	if _, ok := b.topics[topic]; !ok {
		b.topics[topic] = make(map[chan Payload]struct{})
	}
	b.topics[topic][ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		delete(b.topics[topic], ch)
		if len(b.topics[topic]) == 0 {
			delete(b.topics, topic)
		}
		b.mu.Unlock()
		close(ch)
	}

	return ch, unsub
}

func (b *MutexBroker) SubscribeAll() (<-chan TopicPayload, Unsubscribe) {
	ch := make(chan TopicPayload, 16)

	b.mu.Lock()
	b.allSubs[ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		delete(b.allSubs, ch)
		b.mu.Unlock()
		close(ch)
	}

	return ch, unsub
}

func (b *MutexBroker) Publish(topic Topic, payload Payload) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.topics[topic] {
		select {
		case ch <- payload:
		default:
		}
	}
	for ch := range b.allSubs {
		select {
		case ch <- TopicPayload{Topic: topic, Payload: payload}:
		default:
		}
	}
}
