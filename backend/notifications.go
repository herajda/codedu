package main

import (
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	notifTypeAssignmentPublished = "assignment_published"
	notifTypeAssignmentDeadline  = "assignment_deadline"
	notifTypeSecondDeadline      = "assignment_second_deadline"
	notifTypeMessage             = "new_message"
	notificationSweepInterval    = 30 * time.Minute
)

type notificationTarget struct {
	UserID uuid.UUID `db:"id"`
	Email  string    `db:"email"`
	Name   *string   `db:"name"`
}

func (t notificationTarget) displayName() string {
	if t.Name != nil {
		if name := strings.TrimSpace(*t.Name); name != "" {
			return name
		}
	}
	if parts := strings.Split(t.Email, "@"); len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "there"
}

func (t notificationTarget) emailAddress() (string, bool) {
	addr := strings.TrimSpace(t.Email)
	if addr == "" {
		return "", false
	}
	if _, err := mail.ParseAddress(addr); err != nil {
		log.Printf("[notifications] skipping invalid email for user %s: %s", t.UserID, addr)
		return "", false
	}
	return addr, true
}

func notificationAlreadySent(userID uuid.UUID, notifType, context string) (bool, error) {
	var exists bool
	err := DB.Get(&exists, `SELECT EXISTS (SELECT 1 FROM notification_log WHERE user_id=$1 AND notification_type=$2 AND context=$3)`, userID, notifType, context)
	return exists, err
}

func markNotificationSent(userID uuid.UUID, notifType, context string) error {
	_, err := DB.Exec(`INSERT INTO notification_log (user_id, notification_type, context) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, userID, notifType, context)
	return err
}

func listNotificationTargetsForAssignment(aid uuid.UUID) ([]notificationTarget, error) {
	targets := []notificationTarget{}
	err := DB.Select(&targets, `SELECT u.id, u.email, u.name
        FROM assignments a
        JOIN class_students cs ON cs.class_id = a.class_id
        JOIN users u ON u.id = cs.student_id
       WHERE a.id = $1 AND u.email_notifications = TRUE`, aid)
	return targets, err
}

func queueAssignmentPublishedEmail(aid uuid.UUID) {
	if mailer == nil {
		return
	}
	go func(id uuid.UUID) {
		if err := sendAssignmentPublishedNotifications(id); err != nil {
			log.Printf("[notifications] assignment published emails failed: %v", err)
		}
	}(aid)
}

func sendAssignmentPublishedNotifications(aid uuid.UUID) error {
	if mailer == nil {
		return nil
	}
	assignment, err := GetAssignment(aid)
	if err != nil {
		return err
	}
	var className string
	if err := DB.Get(&className, `SELECT name FROM classes WHERE id=$1`, assignment.ClassID); err != nil {
		return err
	}
	targets, err := listNotificationTargetsForAssignment(aid)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	link := ""
	if mailer != nil {
		link = mailer.absoluteURL(fmt.Sprintf("/assignments/%s", aid))
	}
	for _, t := range targets {
		address, ok := t.emailAddress()
		if !ok {
			continue
		}
		sent, err := notificationAlreadySent(t.UserID, notifTypeAssignmentPublished, aid.String())
		if err != nil {
			log.Printf("[notifications] cannot check assignment published log: %v", err)
			continue
		}
		if sent {
			continue
		}
		subject := fmt.Sprintf("New assignment: %s", assignment.Title)
		body := fmt.Sprintf("Hi %s,\n\nA new assignment \"%s\" has been published in %s.\nDeadline: %s.\n",
			t.displayName(), assignment.Title, className, formatTimestamp(assignment.Deadline))
		if assignment.SecondDeadline != nil {
			penalty := (1 - assignment.LatePenaltyRatio) * 100
			if penalty < 0 {
				penalty = 0
			}
			if penalty > 0 {
				body += fmt.Sprintf("Late submissions accepted until %s (penalty %.0f%%).\n",
					formatTimestamp(*assignment.SecondDeadline), penalty)
			} else {
				body += fmt.Sprintf("Late submissions accepted until %s with no penalty.\n",
					formatTimestamp(*assignment.SecondDeadline))
			}
		}
		if link != "" {
			body += fmt.Sprintf("\nView the assignment: %s\n", link)
		}
		body += "\nYou can update your email notification preferences in CodEdu settings.\n"
		if err := mailer.sendPlainText(address, subject, body); err != nil {
			log.Printf("[notifications] failed to send assignment publish email to %s: %v", address, err)
			continue
		}
		if err := markNotificationSent(t.UserID, notifTypeAssignmentPublished, aid.String()); err != nil {
			log.Printf("[notifications] failed to log assignment publish email: %v", err)
		}
	}
	return nil
}

type assignmentDeadlineRow struct {
	ID             uuid.UUID  `db:"id"`
	Title          string     `db:"title"`
	Deadline       time.Time  `db:"deadline"`
	SecondDeadline *time.Time `db:"second_deadline"`
	ClassID        uuid.UUID  `db:"class_id"`
	ClassName      string     `db:"class_name"`
}

var deadlineStages = []struct {
	label     string
	lookahead time.Duration
}{
	{label: "24h", lookahead: 24 * time.Hour},
	{label: "1h", lookahead: time.Hour},
}

func sendAssignmentDeadlineReminders() error {
	if mailer == nil {
		return nil
	}
	now := time.Now()
	assignments := []assignmentDeadlineRow{}
	err := DB.Select(&assignments, `SELECT a.id, a.title, a.deadline, a.second_deadline, a.class_id, c.name AS class_name
         FROM assignments a
         JOIN classes c ON c.id = a.class_id
        WHERE a.published = TRUE
          AND (a.deadline > $1 OR (a.second_deadline IS NOT NULL AND a.second_deadline > $1))`, now)
	if err != nil {
		return err
	}
	for _, asg := range assignments {
		targets, err := listNotificationTargetsForAssignment(asg.ID)
		if err != nil {
			log.Printf("[notifications] cannot list assignment targets: %v", err)
			continue
		}
		if len(targets) == 0 {
			continue
		}
		link := ""
		if mailer != nil {
			link = mailer.absoluteURL(fmt.Sprintf("/assignments/%s", asg.ID))
		}
		if asg.Deadline.After(now) {
			sendDeadlineForTargets(targets, asg, asg.Deadline, link, "primary", notifTypeAssignmentDeadline)
		}
		if asg.SecondDeadline != nil && asg.SecondDeadline.After(now) {
			sendDeadlineForTargets(targets, asg, *asg.SecondDeadline, link, "second", notifTypeSecondDeadline)
		}
	}
	return nil
}

func sendDeadlineForTargets(targets []notificationTarget, asg assignmentDeadlineRow, due time.Time, link, stageName, notifType string) {
	now := time.Now()
	timeUntil := due.Sub(now)
	for _, stage := range deadlineStages {
		if timeUntil > stage.lookahead {
			continue
		}
		context := fmt.Sprintf("%s:%s:%s", asg.ID, stageName, stage.label)
		for _, t := range targets {
			address, ok := t.emailAddress()
			if !ok {
				continue
			}
			sent, err := notificationAlreadySent(t.UserID, notifType, context)
			if err != nil {
				log.Printf("[notifications] cannot check deadline log: %v", err)
				continue
			}
			if sent {
				continue
			}
			subject := fmt.Sprintf("Reminder: %s deadline approaching", asg.Title)
			remaining := formatRelativeDuration(time.Until(due))
			body := fmt.Sprintf("Hi %s,\n\nThis is a reminder that the assignment \"%s\" for %s is due on %s (%s left).\n",
				t.displayName(), asg.Title, asg.ClassName, formatTimestamp(due), remaining)
			if stageName == "second" {
				body += "This refers to the late submission window.\n"
			}
			if link != "" {
				body += fmt.Sprintf("\nSubmit or review your work: %s\n", link)
			}
			body += "\nYou can update your email notification preferences in CodEdu settings.\n"
			if err := mailer.sendPlainText(address, subject, body); err != nil {
				log.Printf("[notifications] failed to send deadline email to %s: %v", address, err)
				continue
			}
			if err := markNotificationSent(t.UserID, notifType, context); err != nil {
				log.Printf("[notifications] failed to log deadline email: %v", err)
			}
		}
	}
}

func queueMessageEmail(msg Message) {
	if mailer == nil {
		return
	}
	go func(m Message) {
		if err := sendMessageNotification(m); err != nil {
			log.Printf("[notifications] message email failed: %v", err)
		}
	}(msg)
}

func sendMessageNotification(msg Message) error {
	if mailer == nil {
		return nil
	}
	if msg.ID == uuid.Nil || msg.SenderID == msg.RecipientID {
		return nil
	}
	recipient, err := GetUser(msg.RecipientID)
	if err != nil {
		return err
	}
	if !recipient.EmailNotifications {
		return nil
	}
	address, ok := (&notificationTarget{UserID: recipient.ID, Email: recipient.Email, Name: recipient.Name}).emailAddress()
	if !ok {
		return nil
	}
	sent, err := notificationAlreadySent(recipient.ID, notifTypeMessage, msg.ID.String())
	if err != nil {
		return err
	}
	if sent {
		return nil
	}
	sender, err := GetUser(msg.SenderID)
	if err != nil {
		return err
	}
	senderName := strings.TrimSpace(sender.Email)
	if sender.Name != nil && strings.TrimSpace(*sender.Name) != "" {
		senderName = strings.TrimSpace(*sender.Name)
	}
	subject := fmt.Sprintf("New message from %s", senderName)
	preview := strings.TrimSpace(msg.Text)
	if preview == "" {
		if msg.Image != nil || msg.File != nil {
			preview = "[Message contains attachments]"
		} else {
			preview = "[No text content]"
		}
	}
	if runes := []rune(preview); len(runes) > 400 {
		preview = string(runes[:400]) + "…"
	}
	preview = strings.ReplaceAll(preview, "\r\n", "\n")
	link := ""
	if mailer != nil {
		link = mailer.absoluteURL(fmt.Sprintf("/messages/%s", msg.SenderID))
	}
	body := fmt.Sprintf("Hi %s,\n\nYou have a new message from %s.\n\nMessage preview:\n%s\n",
		(&notificationTarget{Name: recipient.Name, Email: recipient.Email}).displayName(), senderName, preview)
	if link != "" {
		body += fmt.Sprintf("\nReply now: %s\n", link)
	}
	body += "\nYou can update your email notification preferences in CodEdu settings.\n"
	if err := mailer.sendPlainText(address, subject, body); err != nil {
		return err
	}
	return markNotificationSent(recipient.ID, notifTypeMessage, msg.ID.String())
}

func StartNotificationScheduler() {
	if mailer == nil {
		log.Println("📭 Email notifications disabled; scheduler not started")
		return
	}
	go func() {
		for {
			if err := sendAssignmentDeadlineReminders(); err != nil {
				log.Printf("[notifications] deadline sweep failed: %v", err)
			}
			time.Sleep(notificationSweepInterval)
		}
	}()
}

func formatTimestamp(ts time.Time) string {
	return ts.Local().Format("Mon, 02 Jan 2006 15:04 MST")
}

func formatRelativeDuration(d time.Duration) string {
	if d <= 0 {
		return "due now"
	}
	if d >= 48*time.Hour {
		days := int((d + 12*time.Hour) / (24 * time.Hour))
		if days < 1 {
			days = 1
		}
		if days == 1 {
			return "about 1 day"
		}
		return fmt.Sprintf("about %d days", days)
	}
	if d >= 2*time.Hour {
		hours := int((d + 30*time.Minute) / time.Hour)
		if hours < 1 {
			hours = 1
		}
		return fmt.Sprintf("about %d hours", hours)
	}
	if d >= time.Hour {
		return "about 1 hour"
	}
	minutes := int((d + 30*time.Second) / time.Minute)
	if minutes <= 1 {
		return "about a minute"
	}
	return fmt.Sprintf("about %d minutes", minutes)
}
