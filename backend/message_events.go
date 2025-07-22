package main

import (
	"io"
	"sync"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

type msgSubscriber struct {
	userID int
	ch     chan sse.Event
}

var (
	msgSubsMu sync.Mutex
	msgSubs   = map[*msgSubscriber]bool{}
)

func addMsgSubscriber(uid int) *msgSubscriber {
	sub := &msgSubscriber{userID: uid, ch: make(chan sse.Event, 10)}
	msgSubsMu.Lock()
	msgSubs[sub] = true
	msgSubsMu.Unlock()
	return sub
}

func removeMsgSubscriber(sub *msgSubscriber) {
	msgSubsMu.Lock()
	delete(msgSubs, sub)
	msgSubsMu.Unlock()
	close(sub.ch)
}

func broadcastMsg(evt sse.Event) {
	var uid1, uid2 int
	if m, ok := evt.Data.(*Message); ok {
		uid1 = m.RecipientID
		uid2 = m.SenderID
	}
	msgSubsMu.Lock()
	for sub := range msgSubs {
		if sub.userID == uid1 || sub.userID == uid2 {
			select {
			case sub.ch <- evt:
			default:
			}
		}
	}
	msgSubsMu.Unlock()
}

func messageEventsHandler(c *gin.Context) {
	uid := c.GetInt("userID")
	sub := addMsgSubscriber(uid)
	defer removeMsgSubscriber(sub)
	c.Stream(func(w io.Writer) bool {
		if evt, ok := <-sub.ch; ok {
			c.SSEvent(evt.Event, evt.Data)
			return true
		}
		return false
	})
}
