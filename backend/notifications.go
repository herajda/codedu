package main

import (
	"database/sql"
	"fmt"
	htemplate "html/template"
	"log"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	notifTypeAssignmentPublished = "assignment_published"
	notifTypeAssignmentDeadline  = "assignment_deadline"
	notifTypeSecondDeadline      = "assignment_second_deadline"
	notifTypeMessageDigest       = "message_digest"
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

func buildUnsubscribeURL(userID uuid.UUID, scope string) string {
	if mailer == nil {
		return ""
	}
	token, err := createUnsubscribeToken(userID, scope)
	if err != nil {
		log.Printf("[notifications] cannot create unsubscribe token: %v", err)
		return ""
	}
	path := fmt.Sprintf("/email/unsubscribe?token=%s", url.QueryEscape(token))
	return mailer.absoluteURL(path)
}

func buildTextFooter(unsubURL, description string) string {
	var b strings.Builder
	b.WriteString("\n--\n")
	if strings.TrimSpace(unsubURL) != "" {
		fmt.Fprintf(&b, "To unsubscribe from %s, visit: %s\n", description, unsubURL)
	}
	b.WriteString("You can also manage your email preferences in CodEdu settings.\n")
	return b.String()
}

func wrapHTMLContent(mainHTML, unsubURL, description string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><body style=\"margin:0;background:#f5f7fb;font-family:'Helvetica Neue',Arial,sans-serif;line-height:1.6;color:#1f2933;\">")
	b.WriteString("<div style=\"max-width:600px;margin:0 auto;padding:24px 20px;background:#ffffff;\">")
	b.WriteString(mainHTML)
	if strings.TrimSpace(unsubURL) != "" {
		b.WriteString("<p style=\"margin-top:24px;font-size:13px;color:#5f6b7d;\">To unsubscribe from ")
		b.WriteString(htemplate.HTMLEscapeString(description))
		b.WriteString(", <a style=\"color:#2563eb;text-decoration:none;\" href=\"")
		b.WriteString(htemplate.HTMLEscapeString(unsubURL))
		b.WriteString("\">click here</a>.</p>")
	}
	b.WriteString("<p style=\"margin-top:8px;font-size:13px;color:#5f6b7d;\">You can also manage your email preferences in CodEdu settings.</p>")
	b.WriteString("</div></body></html>")
	return b.String()
}

func buildUnsubscribeHeaders(unsubURL string) map[string]string {
	if strings.TrimSpace(unsubURL) == "" {
		return nil
	}
	return map[string]string{
		"List-Unsubscribe":      fmt.Sprintf("<%s>", unsubURL),
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	}
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
		unsubURL := buildUnsubscribeURL(t.UserID, unsubscribeScopeAlerts)
		textBody, htmlBody := assignmentPublishedEmailBodies(t, assignment, className, link, unsubURL)
		headers := buildUnsubscribeHeaders(unsubURL)
		if err := mailer.sendEmail(address, subject, textBody, htmlBody, headers); err != nil {
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
			unsubURL := buildUnsubscribeURL(t.UserID, unsubscribeScopeAlerts)
			textBody, htmlBody := deadlineEmailBodies(t, asg, due, remaining, stageName, link, unsubURL)
			headers := buildUnsubscribeHeaders(unsubURL)
			if err := mailer.sendEmail(address, subject, textBody, htmlBody, headers); err != nil {
				log.Printf("[notifications] failed to send deadline email to %s: %v", address, err)
				continue
			}
			if err := markNotificationSent(t.UserID, notifType, context); err != nil {
				log.Printf("[notifications] failed to log deadline email: %v", err)
			}
		}
	}
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

	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(next.Sub(now))
			if err := sendDailyMessageDigests(); err != nil {
				log.Printf("[notifications] message digest failed: %v", err)
			}
		}
	}()
}

type messageDigestRow struct {
	SenderID    uuid.UUID `db:"sender_id"`
	SenderName  *string   `db:"sender_name"`
	SenderEmail string    `db:"sender_email"`
	Text        string    `db:"content"`
	Image       *string   `db:"image"`
	FileName    *string   `db:"file_name"`
	CreatedAt   time.Time `db:"created_at"`
}

type messageDigestSummary struct {
	SenderID   uuid.UUID
	SenderName string
	Count      int
	Latest     time.Time
	Samples    []string
}

func sendDailyMessageDigests() error {
	if mailer == nil {
		return nil
	}
	now := time.Now()
	targets := []notificationTarget{}
	if err := DB.Select(&targets, `SELECT id, email, name FROM users WHERE email_notifications = TRUE AND email_message_digest = TRUE`); err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	todayCtx := now.Format("2006-01-02")
	for _, t := range targets {
		address, ok := t.emailAddress()
		if !ok {
			continue
		}
		sentToday, err := notificationAlreadySent(t.UserID, notifTypeMessageDigest, todayCtx)
		if err != nil {
			log.Printf("[notifications] cannot check digest log: %v", err)
			continue
		}
		if sentToday {
			continue
		}

		cutoff := now.Add(-24 * time.Hour)
		var lastSent sql.NullTime
		if err := DB.Get(&lastSent, `SELECT MAX(created_at) FROM notification_log WHERE user_id=$1 AND notification_type=$2`, t.UserID, notifTypeMessageDigest); err != nil {
			log.Printf("[notifications] cannot read last digest time: %v", err)
		} else if lastSent.Valid && lastSent.Time.Before(now) {
			if lastSent.Time.After(cutoff) {
				cutoff = lastSent.Time
			}
		}

		rows := []messageDigestRow{}
		if err := DB.Select(&rows, `SELECT m.sender_id, m.content, m.created_at, m.image, m.file_name, s.name AS sender_name, s.email AS sender_email
             FROM messages m
             JOIN users s ON s.id = m.sender_id
            WHERE m.recipient_id = $1 AND m.created_at > $2 AND m.created_at <= $3
         ORDER BY m.created_at ASC`, t.UserID, cutoff, now); err != nil {
			log.Printf("[notifications] cannot list new messages: %v", err)
			continue
		}
		if len(rows) == 0 {
			continue
		}

		summaries, total := summarizeMessages(rows)
		if total == 0 {
			continue
		}

		subject := fmt.Sprintf("Daily message summary: %d new message%s", total, pluralSuffix(total))
		unsubURL := buildUnsubscribeURL(t.UserID, unsubscribeScopeDigest)
		textBody, htmlBody := buildDigestEmailBodies(t, summaries, total, cutoff, unsubURL)
		if err := mailer.sendEmail(address, subject, textBody, htmlBody, buildUnsubscribeHeaders(unsubURL)); err != nil {
			log.Printf("[notifications] failed to send digest to %s: %v", address, err)
			continue
		}
		if err := markNotificationSent(t.UserID, notifTypeMessageDigest, todayCtx); err != nil {
			log.Printf("[notifications] failed to log digest send: %v", err)
		}
	}
	return nil
}

func summarizeMessages(rows []messageDigestRow) ([]messageDigestSummary, int) {
	bySender := map[uuid.UUID]*messageDigestSummary{}
	total := 0
	for _, row := range rows {
		total++
		sum, ok := bySender[row.SenderID]
		if !ok {
			label := strings.TrimSpace(row.SenderEmail)
			if row.SenderName != nil && strings.TrimSpace(*row.SenderName) != "" {
				label = strings.TrimSpace(*row.SenderName)
			} else if parts := strings.Split(row.SenderEmail, "@"); len(parts) > 0 && parts[0] != "" {
				label = parts[0]
			}
			sum = &messageDigestSummary{SenderID: row.SenderID, SenderName: label}
			bySender[row.SenderID] = sum
		}
		sum.Count++
		if row.CreatedAt.After(sum.Latest) {
			sum.Latest = row.CreatedAt
		}
		preview := buildMessagePreview(row)
		if preview != "" && len(sum.Samples) < 3 {
			sum.Samples = append(sum.Samples, preview)
		}
	}
	summaries := make([]messageDigestSummary, 0, len(bySender))
	for _, s := range bySender {
		summaries = append(summaries, *s)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Latest.After(summaries[j].Latest)
	})
	return summaries, total
}

func buildMessagePreview(row messageDigestRow) string {
	text := strings.TrimSpace(row.Text)
	if text != "" {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	if text == "" {
		if row.FileName != nil && strings.TrimSpace(*row.FileName) != "" {
			text = fmt.Sprintf("[File] %s", strings.TrimSpace(*row.FileName))
		} else if row.Image != nil {
			text = "[Image attachment]"
		} else {
			text = "[No text content]"
		}
	}
	runes := []rune(text)
	if len(runes) > 160 {
		text = string(runes[:160]) + "…"
	}
	return text
}

func assignmentPublishedEmailBodies(target notificationTarget, assignment *Assignment, className, assignmentLink, unsubURL string) (string, string) {
	var textB strings.Builder
	fmt.Fprintf(&textB, "Hi %s,\n\n", target.displayName())
	if assignment != nil {
		fmt.Fprintf(&textB, "A new assignment \"%s\" has been published in %s.\n", assignment.Title, className)
		fmt.Fprintf(&textB, "Deadline: %s.\n", formatTimestamp(assignment.Deadline))
		if assignment.SecondDeadline != nil {
			penalty := (1 - assignment.LatePenaltyRatio) * 100
			if penalty < 0 {
				penalty = 0
			}
			if penalty > 0 {
				fmt.Fprintf(&textB, "Late submissions accepted until %s (penalty %.0f%%).\n", formatTimestamp(*assignment.SecondDeadline), penalty)
			} else {
				fmt.Fprintf(&textB, "Late submissions accepted until %s with no penalty.\n", formatTimestamp(*assignment.SecondDeadline))
			}
		}
	}
	if strings.TrimSpace(assignmentLink) != "" {
		fmt.Fprintf(&textB, "\nView the assignment: %s\n", assignmentLink)
	}
	textB.WriteString(buildTextFooter(unsubURL, "assignment alerts"))

	var htmlB strings.Builder
	htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 16px;\">Hi %s,</p>", htemplate.HTMLEscapeString(target.displayName())))
	if assignment != nil {
		htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 8px;\">A new assignment \"<strong>%s</strong>\" has been published in <strong>%s</strong>.</p>", htemplate.HTMLEscapeString(assignment.Title), htemplate.HTMLEscapeString(className)))
		htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 8px;\">Deadline: <strong>%s</strong>.</p>", htemplate.HTMLEscapeString(formatTimestamp(assignment.Deadline))))
		if assignment.SecondDeadline != nil {
			penalty := (1 - assignment.LatePenaltyRatio) * 100
			if penalty < 0 {
				penalty = 0
			}
			if penalty > 0 {
				htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 8px;\">Late submissions accepted until %s (<strong>%.0f%% penalty</strong>).</p>", htemplate.HTMLEscapeString(formatTimestamp(*assignment.SecondDeadline)), penalty))
			} else {
				htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 8px;\">Late submissions accepted until %s with no penalty.</p>", htemplate.HTMLEscapeString(formatTimestamp(*assignment.SecondDeadline))))
			}
		}
	}
	if strings.TrimSpace(assignmentLink) != "" {
		htmlB.WriteString(fmt.Sprintf("<p style=\"margin:12px 0 16px;\"><a style=\"color:#2563eb;text-decoration:none;\" href=\"%s\">View the assignment</a></p>", htemplate.HTMLEscapeString(assignmentLink)))
	}
	return textB.String(), wrapHTMLContent(htmlB.String(), unsubURL, "assignment alerts")
}

func deadlineEmailBodies(target notificationTarget, asg assignmentDeadlineRow, due time.Time, remaining, stageName, assignmentLink, unsubURL string) (string, string) {
	var textB strings.Builder
	fmt.Fprintf(&textB, "Hi %s,\n\n", target.displayName())
	fmt.Fprintf(&textB, "This is a reminder that the assignment \"%s\" for %s is due on %s (%s left).\n", asg.Title, asg.ClassName, formatTimestamp(due), remaining)
	if stageName == "second" {
		textB.WriteString("This refers to the late submission window.\n")
	}
	if strings.TrimSpace(assignmentLink) != "" {
		fmt.Fprintf(&textB, "\nSubmit or review your work: %s\n", assignmentLink)
	}
	textB.WriteString(buildTextFooter(unsubURL, "assignment alerts"))

	var htmlB strings.Builder
	htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 16px;\">Hi %s,</p>", htemplate.HTMLEscapeString(target.displayName())))
	htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 8px;\">This is a reminder that the assignment \"<strong>%s</strong>\" for <strong>%s</strong> is due on %s (%s left).</p>", htemplate.HTMLEscapeString(asg.Title), htemplate.HTMLEscapeString(asg.ClassName), htemplate.HTMLEscapeString(formatTimestamp(due)), htemplate.HTMLEscapeString(remaining)))
	if stageName == "second" {
		htmlB.WriteString("<p style=\"margin:0 0 8px;color:#b45309;\">This refers to the late submission window.</p>")
	}
	if strings.TrimSpace(assignmentLink) != "" {
		htmlB.WriteString(fmt.Sprintf("<p style=\"margin:12px 0 16px;\"><a style=\"color:#2563eb;text-decoration:none;\" href=\"%s\">Submit or review your work</a></p>", htemplate.HTMLEscapeString(assignmentLink)))
	}
	return textB.String(), wrapHTMLContent(htmlB.String(), unsubURL, "assignment alerts")
}

func buildDigestEmailBodies(target notificationTarget, summaries []messageDigestSummary, total int, since time.Time, unsubURL string) (string, string) {
	link := mailer.absoluteURL("/messages")
	var textB strings.Builder
	fmt.Fprintf(&textB, "Hi %s,\n\n", target.displayName())
	fmt.Fprintf(&textB, "You received %d new message%s since %s.\n\n", total, pluralSuffix(total), formatTimestamp(since))
	for _, s := range summaries {
		fmt.Fprintf(&textB, "- From %s (%d message%s, latest %s)\n", s.SenderName, s.Count, pluralSuffix(s.Count), formatTimestamp(s.Latest))
		for _, sample := range s.Samples {
			fmt.Fprintf(&textB, "   - %s\n", sample)
		}
		textB.WriteString("\n")
	}
	if strings.TrimSpace(link) != "" {
		fmt.Fprintf(&textB, "Review and reply in CodEdu: %s\n", link)
	}
	textB.WriteString(buildTextFooter(unsubURL, "daily message digests"))

	var htmlB strings.Builder
	htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 16px;\">Hi %s,</p>", htemplate.HTMLEscapeString(target.displayName())))
	htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 16px;\">You received %d new message%s since %s.</p>", total, pluralSuffix(total), htemplate.HTMLEscapeString(formatTimestamp(since))))
	htmlB.WriteString("<ul style=\"padding-left:20px;margin:0 0 16px;\">")
	for _, s := range summaries {
		htmlB.WriteString("<li style=\"margin-bottom:12px;\">")
		htmlB.WriteString(fmt.Sprintf("<strong>%s</strong> (%d message%s, latest %s)", htemplate.HTMLEscapeString(s.SenderName), s.Count, pluralSuffix(s.Count), htemplate.HTMLEscapeString(formatTimestamp(s.Latest))))
		if len(s.Samples) > 0 {
			htmlB.WriteString("<ul style=\"padding-left:18px;margin:8px 0 0;\">")
			for _, sample := range s.Samples {
				htmlB.WriteString("<li style=\"margin:4px 0;\">")
				htmlB.WriteString(htemplate.HTMLEscapeString(sample))
				htmlB.WriteString("</li>")
			}
			htmlB.WriteString("</ul>")
		}
		htmlB.WriteString("</li>")
	}
	htmlB.WriteString("</ul>")
	if strings.TrimSpace(link) != "" {
		htmlB.WriteString(fmt.Sprintf("<p style=\"margin:0 0 16px;\"><a style=\"color:#2563eb;text-decoration:none;\" href=\"%s\">Review and reply in CodEdu</a></p>", htemplate.HTMLEscapeString(link)))
	}
	return textB.String(), wrapHTMLContent(htmlB.String(), unsubURL, "daily message digests")
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
