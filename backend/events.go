package main

import (
	"io"
	"sync"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

type subscriber struct {
	userID int
	ch     chan sse.Event
}

var (
	subsMu sync.Mutex
	subs   = map[*subscriber]bool{}
)

func addSubscriber(uid int) *subscriber {
	sub := &subscriber{userID: uid, ch: make(chan sse.Event, 10)}
	subsMu.Lock()
	subs[sub] = true
	subsMu.Unlock()
	return sub
}

func removeSubscriber(sub *subscriber) {
	subsMu.Lock()
	delete(subs, sub)
	subsMu.Unlock()
	close(sub.ch)
}

func broadcast(evt sse.Event) {
	// Determine the submission owner
	var uid int
	var showTrace bool
	switch evt.Event {
	case "status":
		if m, ok := evt.Data.(map[string]any); ok {
			if sid, ok := m["submission_id"].(int); ok {
				if s, err := GetSubmission(sid); err == nil {
					uid = s.StudentID
				}
			}
		}
	case "result":
		if r, ok := evt.Data.(*Result); ok {
			if a, err := GetAssignmentForSubmission(r.SubmissionID); err == nil {
				showTrace = a.ShowTraceback
			}
			if s, err := GetSubmission(r.SubmissionID); err == nil {
				uid = s.StudentID
			}
			if !showTrace {
				r.Stderr = ""
			}
		}
	}
	subsMu.Lock()
	for sub := range subs {
		if uid == 0 || sub.userID == uid {
			select {
			case sub.ch <- evt:
			default:
			}
		}
	}
	subsMu.Unlock()
}

// eventsHandler streams submission updates to clients using SSE.
func eventsHandler(c *gin.Context) {
	uid := c.GetInt("userID")
	sub := addSubscriber(uid)
	defer removeSubscriber(sub)
	// Send current statuses and results so clients don't miss recent updates
	if subs, err := ListSubmissionsForStudent(uid); err == nil {
		for _, s := range subs {
			c.SSEvent("status", map[string]any{"submission_id": s.ID, "status": s.Status})
			if results, err := ListResultsForSubmission(s.ID); err == nil {
				for _, r := range results {
					c.SSEvent("result", r)
				}
			}
		}
	}
	c.Stream(func(w io.Writer) bool {
		if evt, ok := <-sub.ch; ok {
			c.SSEvent(evt.Event, evt.Data)
			return true
		}
		return false
	})
}
