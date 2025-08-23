package main

import (
	"io"
	"strconv"
	"sync"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

type forumSubscriber struct {
	classID int
	ch      chan sse.Event
}

var (
	forumSubsMu sync.Mutex
	forumSubs   = map[*forumSubscriber]bool{}
)

func addForumSubscriber(cid int) *forumSubscriber {
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

func broadcastForumReaction(c *gin.Context, forumMessageID int, reactions []Reaction) {
	// determine class_id of the forum message
	var classID int
	_ = DB.QueryRow(`SELECT class_id FROM forum_messages WHERE id=$1`, forumMessageID).Scan(&classID)
	forumSubsMu.Lock()
	for sub := range forumSubs {
		if sub.classID == classID {
			select {
			case sub.ch <- sse.Event{Event: "reaction", Data: map[string]any{"message_id": forumMessageID, "reactions": reactions}}:
			default:
			}
		}
	}
	forumSubsMu.Unlock()
}

func forumEventsHandler(c *gin.Context) {
	cid, err := strconv.Atoi(c.Param("id"))
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
