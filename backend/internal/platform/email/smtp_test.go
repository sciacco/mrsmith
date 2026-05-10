package email

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestDisabledSendDoesNotDial(t *testing.T) {
	client, err := NewSMTPClient(Config{Enabled: false})
	if err != nil {
		t.Fatalf("NewSMTPClient disabled: %v", err)
	}

	err = client.Send(context.Background(), Message{
		To:      []string{"to@example.com"},
		Subject: "No-op",
		Text:    "No-op",
	})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestNewSMTPClientValidation(t *testing.T) {
	base := Config{
		Enabled:  true,
		Host:     "smtp.example.com",
		Port:     "587",
		Username: "user",
		Password: "pass",
		TLSMode:  TLSModeAuto,
		AuthMode: AuthModeAuto,
	}

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "missing host", cfg: withConfig(base, func(c *Config) { c.Host = "" }), want: "host"},
		{name: "missing username", cfg: withConfig(base, func(c *Config) { c.Username = "" }), want: "username"},
		{name: "missing password", cfg: withConfig(base, func(c *Config) { c.Password = "" }), want: "password"},
		{name: "invalid port", cfg: withConfig(base, func(c *Config) { c.Port = "abc" }), want: "port"},
		{name: "invalid tls mode", cfg: withConfig(base, func(c *Config) { c.TLSMode = "sometimes" }), want: "tls mode"},
		{name: "invalid auth mode", cfg: withConfig(base, func(c *Config) { c.AuthMode = "oauth" }), want: "auth mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSMTPClient(tt.cfg)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestAutoTLSModeUsesImplicitTLSOnPort465(t *testing.T) {
	client, err := NewSMTPClient(Config{
		Enabled:  true,
		Host:     "smtp.example.com",
		Port:     "465",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("NewSMTPClient: %v", err)
	}
	if !client.usesImplicitTLS() {
		t.Fatalf("expected auto TLS mode on port 465 to use implicit TLS")
	}
}

func TestBuildMessageIncludesEnvelopeRecipientsButOmitsBccHeader(t *testing.T) {
	envelope, data, err := buildMessage("Default <sender@example.com>", Message{
		To:      []string{"Recipient <to@example.com>"},
		Cc:      []string{"cc@example.com"},
		Bcc:     []string{"Hidden <hidden@example.com>"},
		ReplyTo: []string{"reply@example.com"},
		Subject: "Hello",
		Text:    "Plain body",
		HTML:    "<p>HTML body</p>",
	})
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	if envelope.from != "sender@example.com" {
		t.Fatalf("unexpected envelope from %q", envelope.from)
	}
	wantRecipients := []string{"to@example.com", "cc@example.com", "hidden@example.com"}
	for _, want := range wantRecipients {
		if !contains(envelope.recipients, want) {
			t.Fatalf("missing envelope recipient %q in %#v", want, envelope.recipients)
		}
	}

	raw := string(data)
	for _, want := range []string{
		`From: "Default" <sender@example.com>`,
		`To: "Recipient" <to@example.com>`,
		"Cc: <cc@example.com>",
		"Reply-To: <reply@example.com>",
		"Subject: Hello",
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Type: text/html; charset=utf-8",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("message data missing %q:\n%s", want, raw)
		}
	}
	if strings.Contains(raw, "hidden@example.com") || strings.Contains(raw, "Bcc:") {
		t.Fatalf("Bcc leaked into message headers:\n%s", raw)
	}
}

func TestSendWithAttachmentsWritesMultipartMixed(t *testing.T) {
	server := startTestSMTPServer(t, testSMTPOptions{
		authMechanisms: "PLAIN",
	})

	client, err := NewSMTPClient(Config{
		Enabled:  true,
		Host:     "localhost",
		Port:     server.port(),
		Username: "user",
		Password: "pass",
		From:     "sender@example.com",
		TLSMode:  TLSModeNone,
		AuthMode: AuthModePlain,
	})
	if err != nil {
		t.Fatalf("NewSMTPClient: %v", err)
	}

	attachmentName := "resume 2026.csv"
	attachmentContent := "id,name\n1,Ada\n"
	err = client.Send(context.Background(), Message{
		To:      []string{"to@example.com"},
		Bcc:     []string{"hidden@example.com"},
		Subject: "With attachment",
		Text:    "Plain body",
		HTML:    "<p>HTML body</p>",
		Attachments: []Attachment{{
			Filename:    attachmentName,
			ContentType: "text/csv",
			Content:     strings.NewReader(attachmentContent),
		}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	session := server.wait(t)
	if !contains(session.rcptTo, "hidden@example.com") {
		t.Fatalf("missing Bcc recipient in envelope: %#v", session.rcptTo)
	}
	if strings.Contains(session.data, "hidden@example.com") || strings.Contains(session.data, "Bcc:") {
		t.Fatalf("Bcc leaked into SMTP DATA:\n%s", session.data)
	}

	msg, err := mail.ReadMessage(strings.NewReader(session.data))
	if err != nil {
		t.Fatalf("read message: %v\n%s", err, session.data)
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("expected multipart/mixed, got %q", mediaType)
	}

	mixed := multipart.NewReader(msg.Body, params["boundary"])
	bodyPart, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("read body part: %v", err)
	}
	bodyMediaType, bodyParams, err := mime.ParseMediaType(bodyPart.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse body content type: %v", err)
	}
	if bodyMediaType != "multipart/alternative" {
		t.Fatalf("expected multipart/alternative body, got %q", bodyMediaType)
	}
	bodyBytes, err := io.ReadAll(bodyPart)
	if err != nil {
		t.Fatalf("read body part: %v", err)
	}

	alternative := multipart.NewReader(bytes.NewReader(bodyBytes), bodyParams["boundary"])
	textPart, err := alternative.NextPart()
	if err != nil {
		t.Fatalf("read text part: %v", err)
	}
	if got := textPart.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected text part content type %q", got)
	}
	textBody, err := io.ReadAll(textPart)
	if err != nil {
		t.Fatalf("read text body: %v", err)
	}
	if !strings.Contains(string(textBody), "Plain body") {
		t.Fatalf("text body missing content: %q", string(textBody))
	}

	htmlPart, err := alternative.NextPart()
	if err != nil {
		t.Fatalf("read html part: %v", err)
	}
	if got := htmlPart.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("unexpected html part content type %q", got)
	}
	htmlBody, err := io.ReadAll(htmlPart)
	if err != nil {
		t.Fatalf("read html body: %v", err)
	}
	if !strings.Contains(string(htmlBody), "<p>HTML body</p>") {
		t.Fatalf("html body missing content: %q", string(htmlBody))
	}
	if _, err := alternative.NextPart(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected two alternative parts, got %v", err)
	}

	attachmentPart, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("read attachment part: %v", err)
	}
	disposition, dispositionParams, err := mime.ParseMediaType(attachmentPart.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parse attachment disposition: %v", err)
	}
	if disposition != "attachment" {
		t.Fatalf("expected attachment disposition, got %q", disposition)
	}
	if dispositionParams["filename"] != attachmentName {
		t.Fatalf("expected attachment filename %q, got %#v", attachmentName, dispositionParams)
	}
	attachmentMediaType, attachmentParams, err := mime.ParseMediaType(attachmentPart.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse attachment content type: %v", err)
	}
	if attachmentMediaType != "text/csv" {
		t.Fatalf("expected text/csv attachment, got %q", attachmentMediaType)
	}
	if attachmentParams["name"] != attachmentName {
		t.Fatalf("expected attachment content-type name %q, got %#v", attachmentName, attachmentParams)
	}
	if got := attachmentPart.Header.Get("Content-Transfer-Encoding"); got != "base64" {
		t.Fatalf("expected base64 transfer encoding, got %q", got)
	}
	encodedAttachment, err := io.ReadAll(attachmentPart)
	if err != nil {
		t.Fatalf("read attachment body: %v", err)
	}
	decodedAttachment, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(string(encodedAttachment))))
	if err != nil {
		t.Fatalf("decode attachment: %v", err)
	}
	if string(decodedAttachment) != attachmentContent {
		t.Fatalf("unexpected attachment content %q", string(decodedAttachment))
	}
	if _, err := mixed.NextPart(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected two mixed parts, got %v", err)
	}
}

func TestBuildMessageAllowsAttachmentOnlyWithDefaultContentType(t *testing.T) {
	_, data, err := buildMessage("sender@example.com", Message{
		To:      []string{"to@example.com"},
		Subject: "Attachment only",
		Attachments: []Attachment{{
			Filename: "payload.bin",
			Content:  strings.NewReader("payload"),
		}},
	})
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	raw := string(data)
	for _, want := range []string{
		"Content-Type: multipart/mixed;",
		"application/octet-stream",
		"Content-Disposition: attachment;",
		"filename=payload.bin",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("message data missing %q:\n%s", want, raw)
		}
	}
}

func TestBuildMessageValidatesAttachments(t *testing.T) {
	tests := []struct {
		name       string
		attachment Attachment
		want       string
	}{
		{
			name:       "missing filename",
			attachment: Attachment{Content: strings.NewReader("data")},
			want:       "filename",
		},
		{
			name:       "filename newline",
			attachment: Attachment{Filename: "bad\nname.txt", Content: strings.NewReader("data")},
			want:       "newline",
		},
		{
			name:       "content type newline",
			attachment: Attachment{Filename: "file.txt", ContentType: "text/plain\nx: y", Content: strings.NewReader("data")},
			want:       "content type",
		},
		{
			name:       "invalid content type",
			attachment: Attachment{Filename: "file.txt", ContentType: "not a content type", Content: strings.NewReader("data")},
			want:       "content type",
		},
		{
			name:       "missing reader",
			attachment: Attachment{Filename: "file.txt"},
			want:       "reader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := buildMessage("sender@example.com", Message{
				To:          []string{"to@example.com"},
				Subject:     "Invalid attachment",
				Text:        "Body",
				Attachments: []Attachment{tt.attachment},
			})
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestBuildMessageFailsWhenAttachmentReaderFails(t *testing.T) {
	_, _, err := buildMessage("sender@example.com", Message{
		To:      []string{"to@example.com"},
		Subject: "Broken attachment",
		Text:    "Body",
		Attachments: []Attachment{{
			Filename: "broken.txt",
			Content:  failingReader{err: errors.New("read failed")},
		}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "broken.txt") || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendWithPlainAuth(t *testing.T) {
	server := startTestSMTPServer(t, testSMTPOptions{
		authMechanisms: "PLAIN LOGIN",
	})

	client, err := NewSMTPClient(Config{
		Enabled:  true,
		Host:     "localhost",
		Port:     server.port(),
		Username: "user",
		Password: "pass",
		From:     "sender@example.com",
		TLSMode:  TLSModeNone,
		AuthMode: AuthModePlain,
	})
	if err != nil {
		t.Fatalf("NewSMTPClient: %v", err)
	}

	err = client.Send(context.Background(), Message{
		To:      []string{"to@example.com"},
		Bcc:     []string{"hidden@example.com"},
		Subject: "Plain auth",
		Text:    "Body",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	session := server.wait(t)
	if session.authUser != "user" || session.authPassword != "pass" {
		t.Fatalf("unexpected auth credentials: %#v", session)
	}
	if session.mailFrom != "sender@example.com" {
		t.Fatalf("unexpected MAIL FROM %q", session.mailFrom)
	}
	for _, want := range []string{"to@example.com", "hidden@example.com"} {
		if !contains(session.rcptTo, want) {
			t.Fatalf("missing RCPT TO %q in %#v", want, session.rcptTo)
		}
	}
	if !strings.Contains(session.data, "Subject: Plain auth") {
		t.Fatalf("message data missing subject:\n%s", session.data)
	}
	if strings.Contains(session.data, "hidden@example.com") || strings.Contains(session.data, "Bcc:") {
		t.Fatalf("Bcc leaked into SMTP DATA:\n%s", session.data)
	}
}

func TestSendWithLoginAuth(t *testing.T) {
	server := startTestSMTPServer(t, testSMTPOptions{
		authMechanisms: "LOGIN",
	})

	client, err := NewSMTPClient(Config{
		Enabled:  true,
		Host:     "localhost",
		Port:     server.port(),
		Username: "login-user",
		Password: "login-pass",
		From:     "sender@example.com",
		TLSMode:  TLSModeNone,
		AuthMode: AuthModeLogin,
	})
	if err != nil {
		t.Fatalf("NewSMTPClient: %v", err)
	}

	err = client.Send(context.Background(), Message{
		To:      []string{"to@example.com"},
		Subject: "Login auth",
		Text:    "Body",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	session := server.wait(t)
	if session.authUser != "login-user" || session.authPassword != "login-pass" {
		t.Fatalf("unexpected login credentials: %#v", session)
	}
}

func TestSendWithStartTLSSkipVerifyToIP(t *testing.T) {
	server := startTestSMTPServer(t, testSMTPOptions{
		authMechanisms: "PLAIN",
		startTLS:       true,
		tlsConfig:      selfSignedTLSConfig(t),
	})

	host, _, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatalf("split server addr: %v", err)
	}
	client, err := NewSMTPClient(Config{
		Enabled:       true,
		Host:          host,
		Port:          server.port(),
		Username:      "user",
		Password:      "pass",
		From:          "sender@example.com",
		TLSMode:       TLSModeStartTLS,
		TLSSkipVerify: true,
		AuthMode:      AuthModePlain,
	})
	if err != nil {
		t.Fatalf("NewSMTPClient: %v", err)
	}

	err = client.Send(context.Background(), Message{
		To:      []string{"to@example.com"},
		Subject: "StartTLS",
		Text:    "Body",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	session := server.wait(t)
	if !session.usedStartTLS {
		t.Fatalf("expected STARTTLS to be used")
	}
	if session.authUser != "user" || session.authPassword != "pass" {
		t.Fatalf("unexpected auth credentials: %#v", session)
	}
}

func withConfig(cfg Config, mutate func(*Config)) Config {
	mutate(&cfg)
	return cfg
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type failingReader struct {
	err error
}

func (r failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

type testSMTPOptions struct {
	authMechanisms string
	startTLS       bool
	tlsConfig      *tls.Config
}

type testSMTPServer struct {
	addr string
	ln   net.Listener
	done chan testSMTPSession
}

type testSMTPSession struct {
	authUser     string
	authPassword string
	mailFrom     string
	rcptTo       []string
	data         string
	usedStartTLS bool
	err          error
}

func startTestSMTPServer(t *testing.T, opts testSMTPOptions) *testSMTPServer {
	t.Helper()
	if opts.authMechanisms == "" {
		opts.authMechanisms = "PLAIN"
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &testSMTPServer{
		addr: ln.Addr().String(),
		ln:   ln,
		done: make(chan testSMTPSession, 1),
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			server.done <- testSMTPSession{err: err}
			return
		}
		defer conn.Close()
		session := handleTestSMTPConn(conn, opts)
		server.done <- session
	}()
	t.Cleanup(func() {
		ln.Close()
	})
	return server
}

func (s *testSMTPServer) port() string {
	_, port, _ := net.SplitHostPort(s.addr)
	return port
}

func (s *testSMTPServer) wait(t *testing.T) testSMTPSession {
	t.Helper()
	select {
	case session := <-s.done:
		if session.err != nil {
			t.Fatalf("smtp test server: %v", session.err)
		}
		return session
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for smtp test server")
		return testSMTPSession{}
	}
}

func handleTestSMTPConn(conn net.Conn, opts testSMTPOptions) testSMTPSession {
	session := testSMTPSession{}
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeLine := func(format string, args ...any) error {
		if _, err := fmt.Fprintf(writer, format+"\r\n", args...); err != nil {
			return err
		}
		return writer.Flush()
	}
	readLine := func() (string, error) {
		line, err := reader.ReadString('\n')
		return strings.TrimRight(line, "\r\n"), err
	}

	if err := writeLine("220 localhost ESMTP"); err != nil {
		session.err = err
		return session
	}

	for {
		line, err := readLine()
		if err != nil {
			session.err = err
			return session
		}
		upper := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(upper, "EHLO ") || strings.HasPrefix(upper, "HELO "):
			if opts.startTLS && !session.usedStartTLS {
				if err := writeLine("250-localhost"); err != nil {
					session.err = err
					return session
				}
				if err := writeLine("250-STARTTLS"); err != nil {
					session.err = err
					return session
				}
				if err := writeLine("250 AUTH %s", opts.authMechanisms); err != nil {
					session.err = err
					return session
				}
			} else {
				if err := writeLine("250-localhost"); err != nil {
					session.err = err
					return session
				}
				if err := writeLine("250 AUTH %s", opts.authMechanisms); err != nil {
					session.err = err
					return session
				}
			}
		case upper == "STARTTLS":
			if !opts.startTLS || opts.tlsConfig == nil {
				if err := writeLine("454 TLS not available"); err != nil {
					session.err = err
					return session
				}
				continue
			}
			if err := writeLine("220 Ready to start TLS"); err != nil {
				session.err = err
				return session
			}
			tlsConn := tls.Server(conn, opts.tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				session.err = err
				return session
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
			session.usedStartTLS = true
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			user, pass, err := decodePlainAuth(line)
			if err != nil {
				session.err = err
				return session
			}
			session.authUser = user
			session.authPassword = pass
			if err := writeLine("235 Authentication successful"); err != nil {
				session.err = err
				return session
			}
		case strings.HasPrefix(upper, "AUTH LOGIN"):
			user, pass, err := handleLoginAuth(readLine, writeLine)
			if err != nil {
				session.err = err
				return session
			}
			session.authUser = user
			session.authPassword = pass
		case strings.HasPrefix(upper, "MAIL FROM:"):
			session.mailFrom = extractSMTPPath(line)
			if err := writeLine("250 OK"); err != nil {
				session.err = err
				return session
			}
		case strings.HasPrefix(upper, "RCPT TO:"):
			session.rcptTo = append(session.rcptTo, extractSMTPPath(line))
			if err := writeLine("250 OK"); err != nil {
				session.err = err
				return session
			}
		case upper == "DATA":
			if err := writeLine("354 End data with <CR><LF>.<CR><LF>"); err != nil {
				session.err = err
				return session
			}
			data, err := readSMTPData(readLine)
			if err != nil {
				session.err = err
				return session
			}
			session.data = data
			if err := writeLine("250 OK"); err != nil {
				session.err = err
				return session
			}
		case upper == "QUIT":
			if err := writeLine("221 Bye"); err != nil {
				session.err = err
			}
			return session
		default:
			session.err = fmt.Errorf("unexpected SMTP command %q", line)
			return session
		}
	}
}

func decodePlainAuth(line string) (string, string, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", "", fmt.Errorf("unexpected AUTH PLAIN command %q", line)
	}
	decoded, err := base64.StdEncoding.DecodeString(fields[2])
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("unexpected AUTH PLAIN payload %q", string(decoded))
	}
	return parts[1], parts[2], nil
}

func handleLoginAuth(readLine func() (string, error), writeLine func(string, ...any) error) (string, string, error) {
	if err := writeLine("334 %s", base64.StdEncoding.EncodeToString([]byte("Username:"))); err != nil {
		return "", "", err
	}
	userLine, err := readLine()
	if err != nil {
		return "", "", err
	}
	user, err := base64.StdEncoding.DecodeString(userLine)
	if err != nil {
		return "", "", err
	}

	if err := writeLine("334 %s", base64.StdEncoding.EncodeToString([]byte("Password:"))); err != nil {
		return "", "", err
	}
	passLine, err := readLine()
	if err != nil {
		return "", "", err
	}
	pass, err := base64.StdEncoding.DecodeString(passLine)
	if err != nil {
		return "", "", err
	}
	if err := writeLine("235 Authentication successful"); err != nil {
		return "", "", err
	}
	return string(user), string(pass), nil
}

func extractSMTPPath(line string) string {
	start := strings.Index(line, "<")
	end := strings.LastIndex(line, ">")
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	_, value, _ := strings.Cut(line, ":")
	return strings.TrimSpace(value)
}

func readSMTPData(readLine func() (string, error)) (string, error) {
	var lines []string
	for {
		line, err := readLine()
		if err != nil {
			return "", err
		}
		if line == "." {
			return strings.Join(lines, "\n"), nil
		}
		if strings.HasPrefix(line, "..") {
			line = line[1:]
		}
		lines = append(lines, line)
	}
}

func selfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "smtp-test.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}
