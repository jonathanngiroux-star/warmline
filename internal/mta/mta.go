// Package mta is the local SMTP submission front. It accepts a local
// SMTP submit and hands the message to the queue via the Handler hook.
// It is deliberately NOT a full MTA: no routing, no DNS MX lookups, no
// outbound delivery. Outbound goes through user-supplied relay
// credentials — Warmline never resells email.
package mta

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

// Message is a submitted message as captured from the SMTP dialogue.
type Message struct {
	From string
	To   []string
	Data string
}

// Handler receives each accepted message. The queue store implements it.
type Handler interface {
	Submit(m Message) error
}

// Server is the SMTP submission listener.
type Server struct {
	mu      sync.Mutex
	ln      net.Listener
	handler Handler
	inbox   []Message // test/inspection path
}

// New returns a Server with no handler; call SetHandler before Serve.
func New() *Server { return &Server{} }

// SetHandler wires the submit handler (queue store).
func (s *Server) SetHandler(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = h
}

// Listen binds addr (use :0 for a test port).
func (s *Server) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	return nil
}

// Addr returns the bound address (host:port).
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Serve accepts connections until Close. Run in its own goroutine
// (go srv.Serve()).
func (s *Server) Serve() error {
	for {
		s.mu.Lock()
		ln := s.ln
		s.mu.Unlock()
		if ln == nil {
			return fmt.Errorf("mta: Serve before Listen")
		}
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

// Close stops the listener.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	err := s.ln.Close()
	s.ln = nil
	return err
}

// Inbox returns accepted messages (drains). Test/inspection path; the
// real consumer is the queue via Handler.
func (s *Server) Inbox() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.inbox))
	copy(out, s.inbox)
	s.inbox = nil
	return out
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	writeLine := func(line string) {
		fmt.Fprintf(w, "%s\r\n", line)
		w.Flush()
	}
	writeLine("220 warmline local submission ready")

	// Per-connection session state (RFC 5321): never on Server —
	// connections run concurrently and must not share dialogue state.
	var (
		from string
		rcpt []string
	)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		upper := strings.ToUpper(cmd)
		verb := upper
		if i := strings.IndexAny(upper, " 	"); i > 0 {
			verb = upper[:i]
		}
		switch verb {
		case "EHLO", "HELO":
			writeLine("250-warmline")
			writeLine("250 OK")
		case "MAIL":
			// Address is case-sensitive per RFC 5321; only the verb was
			// uppercased. Extract from the raw line.
			arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cmd), "MAIL FROM:"))
			arg = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(arg), "<"), ">")
			if arg == "" {
				writeLine("501 missing address")
				continue
			}
			from = arg
			writeLine("250 OK")
		case "RCPT":
			if from == "" {
				writeLine("503 need MAIL before RCPT")
				continue
			}
			arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cmd), "RCPT TO:"))
			arg = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(arg), "<"), ">")
			if arg == "" {
				writeLine("501 missing address")
				continue
			}
			rcpt = append(rcpt, arg)
			writeLine("250 OK")
		case "DATA":
			if len(rcpt) == 0 {
				writeLine("503 need RCPT before DATA")
				continue
			}
			writeLine("354 end with <CRLF>.<CRLF>")
			var data strings.Builder
			for {
				dl, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(dl) == "." {
					break
				}
				data.WriteString(dl)
			}
			msg := Message{From: from, To: rcpt, Data: data.String()}
			s.mu.Lock()
			s.inbox = append(s.inbox, msg)
			h := s.handler
			s.mu.Unlock()
			if h != nil {
				_ = h.Submit(msg)
			}
			// reset per-message state per RFC 5321
			from = ""
			rcpt = nil
			writeLine("250 OK queued")
		case "RSET":
			from = ""
			rcpt = nil
			writeLine("250 OK")
		case "NOOP":
			writeLine("250 OK")
		case "QUIT":
			writeLine("221 bye")
			return
		default:
			writeLine("502 command not implemented")
		}
	}
}
