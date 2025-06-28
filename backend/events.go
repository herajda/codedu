package main

import (
	"github.com/gin-contrib/sse"
	"sync"
)

type subscriber chan sse.Event

var (
	subsMu sync.Mutex
	subs   = map[subscriber]bool{}
)

func addSubscriber() subscriber {
	ch := make(subscriber, 10)
	subsMu.Lock()
	subs[ch] = true
	subsMu.Unlock()
	return ch
}

func removeSubscriber(ch subscriber) {
	subsMu.Lock()
	delete(subs, ch)
	subsMu.Unlock()
	close(ch)
}

func broadcast(evt sse.Event) {
	subsMu.Lock()
	for ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
	subsMu.Unlock()
}
