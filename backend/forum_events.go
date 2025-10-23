package main

import (
	"io"
	"sync"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type forumSubscriber struct {
	classID uuid.UUID
	ch      chan sse.Event
}

var (
	forumSubsMu sync.Mutex
	forumSubs   = map[*forumSubscriber]bool{}
)

func addForumSubscriber(cid uuid.UUID) *forumSubscriber {
	sub := &forumSubscriber{classID: cid, ch: make(chan sse.Event, 10)}
	forumSubsMu.Lock()
	forumSubs[sub] = true
	forumSubsMu.Unlock()
	return sub
}

func removeForumSubscriber(sub *forumSubscriber) {
	forumSubsMu.Lock()
	delete(forumSubs, sub)
	forumSubsMu.Unlock()
	close(sub.ch)
}

func broadcastForumMsg(m *ForumMessage) {
	forumSubsMu.Lock()
	for sub := range forumSubs {
		if sub.classID == m.ClassID {
			select {
			case sub.ch <- sse.Event{Event: "message", Data: m}:
			default:
			}
		}
	}
	forumSubsMu.Unlock()
}

func broadcastForumDelete(classID, messageID uuid.UUID) {
	payload := struct {
		ID uuid.UUID `json:"id"`
	}{ID: messageID}
	forumSubsMu.Lock()
	for sub := range forumSubs {
		if sub.classID == classID {
			select {
			case sub.ch <- sse.Event{Event: "deleted", Data: payload}:
			default:
			}
		}
	}
	forumSubsMu.Unlock()
}

func forumEventsHandler(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	if !ensureClassMember(c, cid) {
		return
	}
	sub := addForumSubscriber(cid)
	defer removeForumSubscriber(sub)
	c.Stream(func(w io.Writer) bool {
		if evt, ok := <-sub.ch; ok {
			c.SSEvent(evt.Event, evt.Data)
			return true
		}
		return false
	})
}
