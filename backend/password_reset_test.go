package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/mail"
	"strings"
	"testing"
)

func TestBuildPlainTextMessageHeaders(t *testing.T) {
	m := &mailerConfig{
		fromAddress:     "noreply@example.com",
		fromHeader:      "CodeEdu <noreply@example.com>",
		messageIDDomain: "example.com",
	}

	msg, err := m.buildPlainTextMessage("Student <student@example.net>", "Reset", "Body line one\nBody line two")
	if err != nil {
		t.Fatalf("buildPlainTextMessage error: %v", err)
	}

	parsed, err := mail.ReadMessage(bufio.NewReader(bytes.NewReader(msg)))
	if err != nil {
		t.Fatalf("failed to parse email message: %v", err)
	}

	if parsed.Header.Get("Date") == "" {
		t.Fatal("expected Date header to be present")
	}

	fromList, err := mail.ParseAddressList(parsed.Header.Get("From"))
	if err != nil {
		t.Fatalf("failed to parse From header: %v", err)
	}
	if len(fromList) != 1 || fromList[0].Address != "noreply@example.com" || fromList[0].Name != "CodeEdu" {
		t.Fatalf("unexpected From header: %#v", fromList)
	}

	toList, err := mail.ParseAddressList(parsed.Header.Get("To"))
	if err != nil {
		t.Fatalf("failed to parse To header: %v", err)
	}
	if len(toList) != 1 || toList[0].Address != "student@example.net" || toList[0].Name != "Student" {
		t.Fatalf("unexpected To header: %#v", toList)
	}

	msgID := parsed.Header.Get("Message-Id")
	if msgID == "" {
		t.Fatal("expected Message-ID header to be present")
	}
	if !strings.HasSuffix(msgID, "@example.com>") {
		t.Fatalf("Message-ID domain mismatch: %s", msgID)
	}

	if got := parsed.Header.Get("Mime-Version"); got != "1.0" {
		t.Fatalf("unexpected MIME-Version header: %q", got)
	}

	if got := parsed.Header.Get("Content-Type"); got != "text/plain; charset=UTF-8" {
		t.Fatalf("unexpected Content-Type header: %q", got)
	}

	if got := parsed.Header.Get("Content-Transfer-Encoding"); got != "8bit" {
		t.Fatalf("unexpected Content-Transfer-Encoding header: %q", got)
	}

	body, err := io.ReadAll(parsed.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !bytes.Contains(body, []byte("Body line one\r\nBody line two\r\n")) {
		t.Fatalf("unexpected body payload: %q", string(body))
	}
}

func TestSignMessageAddsDKIMSignature(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	m := &mailerConfig{
		fromAddress:     "noreply@example.com",
		fromHeader:      "CodeEdu <noreply@example.com>",
		messageIDDomain: "example.com",
		dkimDomain:      "example.com",
		dkimSelector:    "test",
		dkimSigner:      priv,
		dkimHeaderKeys:  defaultDKIMHeaderKeys(),
	}

	rawMsg, err := m.buildPlainTextMessage("student@example.net", "Reset", "Body")
	if err != nil {
		t.Fatalf("buildPlainTextMessage error: %v", err)
	}

	signed, err := m.signMessage(rawMsg)
	if err != nil {
		t.Fatalf("signMessage error: %v", err)
	}

	if !bytes.HasPrefix(signed, []byte("DKIM-Signature:")) {
		preview := signed
		if len(preview) > 64 {
			preview = preview[:64]
		}
		t.Fatalf("expected DKIM-Signature header, got: %q", preview)
	}

	if !bytes.Contains(signed, []byte(" d=example.com")) {
		t.Fatalf("DKIM signature missing domain: %q", signed)
	}

	if !bytes.Contains(signed, []byte(" s=test")) {
		t.Fatalf("DKIM signature missing selector: %q", signed)
	}
}
