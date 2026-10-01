package mta

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

func startServer(t *testing.T) (*Server, string) {
	t.Helper()
	srv := New()
	if err := srv.Listen("127.0.0.1:0"); err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Close() })
	return srv, srv.Addr()
}

// dial opens a connection and reads the greeting line.
func dial(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	r := bufio.NewReader(conn)
	greeting := readLine(t, r)
	if !strings.HasPrefix(greeting, "220") {
		t.Fatalf("bad greeting: %q", greeting)
	}
	return conn, r
}

func sendCmd(t *testing.T, conn net.Conn, r *bufio.Reader, cmd string) string {
	t.Helper()
	if _, err := conn.Write([]byte(cmd + "\r\n")); err != nil {
		t.Fatalf("write %q: %v", cmd, err)
	}
	return readLine(t, r)
}

func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return strings.TrimRight(line, "\r\n")
}

func TestSMTPSubmitEnqueues(t *testing.T) {
	srv, addr := startServer(t)
	conn, r := dial(t, addr)

	if resp := sendCmd(t, conn, r, "EHLO tester"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("EHLO: %q", resp)
	}
	// multi-line EHLO response
	for {
		line := readLine(t, r)
		if !strings.HasPrefix(line, "250-") {
			break
		}
	}
	if resp := sendCmd(t, conn, r, "MAIL FROM:<app@example.com>"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("MAIL FROM: %q", resp)
	}
	if resp := sendCmd(t, conn, r, "RCPT TO:<user@example.net>"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("RCPT TO: %q", resp)
	}
	if resp := sendCmd(t, conn, r, "DATA"); !strings.HasPrefix(resp, "354") {
		t.Fatalf("DATA: %q", resp)
	}
	body := "Subject: hi\r\n\r\nhello world\r\n.\r\n"
	if _, err := conn.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	resp := readLine(t, r)
	if !strings.HasPrefix(resp, "250") {
		t.Fatalf("after DATA: %q", resp)
	}
	_ = sendCmd(t, conn, r, "QUIT")

	// The message must be in the in-memory inbox the test server keeps.
	msgs := srv.Inbox()
	if len(msgs) != 1 {
		t.Fatalf("inbox = %d messages, want 1", len(msgs))
	}
	m := msgs[0]
	if m.From != "app@example.com" {
		t.Errorf("from = %q", m.From)
	}
	if len(m.To) != 1 || m.To[0] != "user@example.net" {
		t.Errorf("to = %v", m.To)
	}
	if !strings.Contains(m.Data, "hello world") {
		t.Errorf("data missing body: %q", m.Data)
	}
}

func TestSMTPRejectsDataWithoutRcpt(t *testing.T) {
	_, addr := startServer(t)
	conn, r := dial(t, addr)
	sendCmd(t, conn, r, "EHLO tester")
	for strings.HasPrefix(readLine(t, r), "250-") {
	}
	sendCmd(t, conn, r, "MAIL FROM:<app@example.com>")
	resp := sendCmd(t, conn, r, "DATA")
	if !strings.HasPrefix(resp, "5") {
		t.Fatalf("DATA without RCPT must be rejected 5xx, got %q", resp)
	}
}

func TestSMTPRejectsMailWithoutFrom(t *testing.T) {
	_, addr := startServer(t)
	conn, r := dial(t, addr)
	sendCmd(t, conn, r, "EHLO tester")
	for strings.HasPrefix(readLine(t, r), "250-") {
	}
	resp := sendCmd(t, conn, r, "RCPT TO:<user@example.net>")
	if !strings.HasPrefix(resp, "5") {
		t.Fatalf("RCPT without MAIL must be rejected 5xx, got %q", resp)
	}
}

func TestSMTPNoopQuit(t *testing.T) {
	_, addr := startServer(t)
	conn, r := dial(t, addr)
	sendCmd(t, conn, r, "EHLO tester")
	for strings.HasPrefix(readLine(t, r), "250-") {
	}
	if resp := sendCmd(t, conn, r, "NOOP"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("NOOP: %q", resp)
	}
	resp := sendCmd(t, conn, r, "QUIT")
	if !strings.HasPrefix(resp, "221") {
		t.Fatalf("QUIT: %q", resp)
	}
}

// RFC 5321 §4.5.2: a line starting with "." in the body is sent stuffed
// (".." on the wire) and MUST be un-stuffed by the receiver. Storing the
// stuffed form corrupts the body: relaying re-stuffs it.
func TestSMTPDataUnstuffsLeadingDots(t *testing.T) {
	srv, addr := startServer(t)
	conn, r := dial(t, addr)
	defer conn.Close()
	sendCmd(t, conn, r, "EHLO t")
	for strings.HasPrefix(readLine(t, r), "250-") {
	}
	sendCmd(t, conn, r, "MAIL FROM:<app@example.com>")
	sendCmd(t, conn, r, "RCPT TO:<user@example.net>")
	sendCmd(t, conn, r, "DATA")
	// body contains a line that starts with a single dot
	body := "Subject: dot test\r\n\r\n..stuffed line\r\nnormal line\r\n.\r\n"
	if _, err := conn.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	resp := readLine(t, r)
	if !strings.HasPrefix(resp, "250") {
		t.Fatalf("DATA submit: %q", resp)
	}
	inbox := srv.Inbox()
	if len(inbox) != 1 {
		t.Fatalf("inbox = %d, want 1", len(inbox))
	}
	if strings.Contains(inbox[0].Data, "..stuffed") {
		t.Errorf("body stored STUFFED — receiver must un-stuff: %q", inbox[0].Data)
	}
	if !strings.Contains(inbox[0].Data, ".stuffed line") {
		t.Errorf("un-stuffed body line missing: %q", inbox[0].Data)
	}
}

// RFC 5321 verbs are case-insensitive; lowercase commands must parse.
func TestSMTPLowercaseCommands(t *testing.T) {
	srv, addr := startServer(t)
	conn, r := dial(t, addr)
	defer conn.Close()
	sendCmd(t, conn, r, "ehlo tester")
	for strings.HasPrefix(readLine(t, r), "250-") {
	}
	if resp := sendCmd(t, conn, r, "mail from:<app@example.com>"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("lowercase MAIL: %q", resp)
	}
	if resp := sendCmd(t, conn, r, "rcpt to:<user@example.net>"); !strings.HasPrefix(resp, "250") {
		t.Fatalf("lowercase RCPT: %q", resp)
	}
	sendCmd(t, conn, r, "DATA")
	if _, err := conn.Write([]byte("From: app@example.com\r\n\r\nhi\r\n.\r\n")); err != nil {
		t.Fatal(err)
	}
	if resp := readLine(t, r); !strings.HasPrefix(resp, "250") {
		t.Fatalf("lowercase submit: %q", resp)
	}
	inbox := srv.Inbox()
	if len(inbox) != 1 {
		t.Fatalf("inbox = %d, want 1", len(inbox))
	}
	if inbox[0].From != "app@example.com" {
		t.Errorf("From = %q (garbage from case-sensitive trim), want app@example.com", inbox[0].From)
	}
	if inbox[0].To[0] != "user@example.net" {
		t.Errorf("To = %v, want user@example.net", inbox[0].To)
	}
}
