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
	} else if evt.Event == "reaction" {
		if m, ok := evt.Data.(map[string]any); ok {
			// best effort: deliver to both ends of the conversation by looking up message
			if mid, ok := m["message_id"].(int); ok {
				var sID, rID int
				_ = DB.QueryRow(`SELECT sender_id, recipient_id FROM messages WHERE id=$1`, mid).Scan(&sID, &rID)
				uid1 = rID
				uid2 = sID
			}
		}
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

func broadcastRead(senderID, readerID int) {
	msgSubsMu.Lock()
	for sub := range msgSubs {
		if sub.userID == senderID {
			select {
			case sub.ch <- sse.Event{Event: "read", Data: map[string]int{"reader_id": readerID}}:
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
