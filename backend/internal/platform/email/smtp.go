package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultPort    = "587"
	defaultTimeout = 30 * time.Second

	TLSModeAuto     = "auto"
	TLSModeStartTLS = "starttls"
	TLSModeImplicit = "implicit"
	TLSModeNone     = "none"

	AuthModeAuto  = "auto"
	AuthModePlain = "plain"
	AuthModeLogin = "login"

	defaultAttachmentContentType = "application/octet-stream"
	base64LineLength             = 76
)

var ErrDisabled = errors.New("email disabled")

type Config struct {
	Enabled       bool
	Host          string
	Port          string
	Username      string
	Password      string
	From          string
	TLSMode       string
	TLSSkipVerify bool
	TLSServerName string
	AuthMode      string
	Timeout       time.Duration
}

type Message struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	ReplyTo     []string
	Subject     string
	Text        string
	HTML        string
	Attachments []Attachment
}

type Attachment struct {
	Filename    string
	ContentType string
	Content     io.Reader
}

type Client struct {
	cfg Config
}

func NewSMTPClient(cfg Config) (*Client, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{cfg: normalized}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled
}

func (c *Client) Send(ctx context.Context, msg Message) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	if ctx == nil {
		ctx = context.Background()
	}

	message, err := prepareMessage(c.cfg.From, msg)
	if err != nil {
		return err
	}

	client, conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	stopWatch := c.watchContext(ctx, conn)
	defer stopWatch()

	c.setDeadline(ctx, conn)
	if err := c.startTLSIfNeeded(client); err != nil {
		return err
	}
	c.setDeadline(ctx, conn)
	if err := c.authenticate(client); err != nil {
		return err
	}
	c.setDeadline(ctx, conn)
	if err := client.Mail(message.envelope.from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, recipient := range message.envelope.recipients {
		c.setDeadline(ctx, conn)
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp rcpt to: %w", err)
		}
	}
	c.setDeadline(ctx, conn)
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	c.setDeadline(ctx, conn)
	if err := writeMessage(writer, message); err != nil {
		writer.Close()
		return fmt.Errorf("smtp write message: %w", err)
	}
	c.setDeadline(ctx, conn)
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	c.setDeadline(ctx, conn)
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

// setDeadline applies a per-phase I/O deadline to conn, capped by ctx.Deadline().
// Without this, a single dial-time deadline would have to cover the entire
// session (TLS, auth, RCPTs, body write, QUIT) — large attachments can blow it.
func (c *Client) setDeadline(ctx context.Context, conn net.Conn) {
	deadline := time.Now().Add(c.cfg.Timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
}

// watchContext closes conn when ctx is cancelled, so Send aborts mid-flight
// instead of running until the next deadline tick. The returned stop function
// must be called to release the watcher goroutine.
func (c *Client) watchContext(ctx context.Context, conn net.Conn) func() {
	if ctx.Done() == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func normalizeConfig(cfg Config) (Config, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.From = strings.TrimSpace(cfg.From)
	cfg.TLSMode = strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	cfg.TLSServerName = strings.TrimSpace(cfg.TLSServerName)
	cfg.AuthMode = strings.ToLower(strings.TrimSpace(cfg.AuthMode))
	if cfg.Port == "" {
		cfg.Port = DefaultPort
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = TLSModeAuto
	}
	if cfg.AuthMode == "" {
		cfg.AuthMode = AuthModeAuto
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	port, err := parsePort(cfg.Port)
	if err != nil {
		return Config{}, err
	}
	cfg.Port = strconv.Itoa(port)
	if cfg.Host == "" {
		return Config{}, errors.New("smtp host is required")
	}
	if cfg.Username == "" {
		return Config{}, errors.New("smtp username is required")
	}
	if cfg.Password == "" {
		return Config{}, errors.New("smtp password is required")
	}
	switch cfg.TLSMode {
	case TLSModeAuto, TLSModeStartTLS, TLSModeImplicit, TLSModeNone:
	default:
		return Config{}, fmt.Errorf("unsupported smtp tls mode %q", cfg.TLSMode)
	}
	switch cfg.AuthMode {
	case AuthModeAuto, AuthModePlain, AuthModeLogin:
	default:
		return Config{}, fmt.Errorf("unsupported smtp auth mode %q", cfg.AuthMode)
	}
	return cfg, nil
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid smtp port %q", raw)
	}
	return port, nil
}

func (c *Client) dial(ctx context.Context) (*smtp.Client, net.Conn, error) {
	address := net.JoinHostPort(c.cfg.Host, c.cfg.Port)
	var (
		conn net.Conn
		err  error
	)
	if c.usesImplicitTLS() {
		dialer := tls.Dialer{
			NetDialer: &net.Dialer{Timeout: c.cfg.Timeout},
			Config:    c.tlsConfig(),
		}
		conn, err = dialer.DialContext(ctx, "tcp", address)
	} else {
		dialer := net.Dialer{Timeout: c.cfg.Timeout}
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("smtp dial %s: %w", address, err)
	}

	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("smtp client: %w", err)
	}
	return client, conn, nil
}

func (c *Client) startTLSIfNeeded(client *smtp.Client) error {
	switch c.cfg.TLSMode {
	case TLSModeImplicit, TLSModeNone:
		return nil
	case TLSModeStartTLS:
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not support STARTTLS")
		}
		if err := client.StartTLS(c.tlsConfig()); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	case TLSModeAuto:
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(c.tlsConfig()); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	return nil
}

func (c *Client) authenticate(client *smtp.Client) error {
	auth, err := c.auth(client)
	if err != nil {
		return err
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return nil
}

func (c *Client) auth(client *smtp.Client) (smtp.Auth, error) {
	switch c.cfg.AuthMode {
	case AuthModePlain:
		return smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host), nil
	case AuthModeLogin:
		return &loginAuth{username: c.cfg.Username, password: c.cfg.Password}, nil
	case AuthModeAuto:
		if supportsAuth(client, "PLAIN") {
			return smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host), nil
		}
		if supportsAuth(client, "LOGIN") {
			return &loginAuth{username: c.cfg.Username, password: c.cfg.Password}, nil
		}
		return nil, errors.New("smtp server does not support PLAIN or LOGIN auth")
	default:
		return nil, fmt.Errorf("unsupported smtp auth mode %q", c.cfg.AuthMode)
	}
}

func supportsAuth(client *smtp.Client, mechanism string) bool {
	ok, params := client.Extension("AUTH")
	if !ok {
		return false
	}
	mechanism = strings.ToUpper(strings.TrimSpace(mechanism))
	for _, field := range strings.Fields(strings.ToUpper(strings.ReplaceAll(params, "=", " "))) {
		if field == mechanism {
			return true
		}
	}
	return false
}

func (c *Client) usesImplicitTLS() bool {
	if c.cfg.TLSMode == TLSModeImplicit {
		return true
	}
	return c.cfg.TLSMode == TLSModeAuto && c.cfg.Port == "465"
}

func (c *Client) tlsConfig() *tls.Config {
	serverName := c.cfg.TLSServerName
	if serverName == "" {
		serverName = strings.Trim(c.cfg.Host, "[]")
	}
	return &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: c.cfg.TLSSkipVerify,
	}
}

type loginAuth struct {
	username string
	password string
	step     int
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, errors.New("unencrypted connection")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch a.step {
	case 0:
		a.step++
		return []byte(a.username), nil
	case 1:
		a.step++
		return []byte(a.password), nil
	default:
		return nil, errors.New("unexpected LOGIN auth challenge")
	}
}

func isLocalhost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

type smtpEnvelope struct {
	from       string
	recipients []string
}

type smtpMessage struct {
	envelope     smtpEnvelope
	fromHeader   string
	senderHeader string
	toHeader     string
	ccHeader     string
	replyTo      string
	subject      string
	date         string
	messageID    string
	text         string
	html         string
	attachments  []Attachment
}

func prepareMessage(defaultFrom string, msg Message) (smtpMessage, error) {
	explicitFrom := strings.TrimSpace(msg.From)
	from := explicitFrom
	if from == "" {
		from = strings.TrimSpace(defaultFrom)
	}
	fromAddr, err := parseSingleAddress("from", from)
	if err != nil {
		return smtpMessage{}, err
	}

	toHeader, toRecipients, err := parseAddressList("to", msg.To)
	if err != nil {
		return smtpMessage{}, err
	}
	ccHeader, ccRecipients, err := parseAddressList("cc", msg.Cc)
	if err != nil {
		return smtpMessage{}, err
	}
	_, bccRecipients, err := parseAddressList("bcc", msg.Bcc)
	if err != nil {
		return smtpMessage{}, err
	}
	replyToHeader, _, err := parseAddressList("reply-to", msg.ReplyTo)
	if err != nil {
		return smtpMessage{}, err
	}

	recipients := append(append(toRecipients, ccRecipients...), bccRecipients...)
	if len(recipients) == 0 {
		return smtpMessage{}, errors.New("email message requires at least one recipient")
	}
	if strings.TrimSpace(msg.Text) == "" && strings.TrimSpace(msg.HTML) == "" && len(msg.Attachments) == 0 {
		return smtpMessage{}, errors.New("email message requires text or html body or at least one attachment")
	}
	if hasHeaderBreak(msg.Subject) {
		return smtpMessage{}, errors.New("email subject contains invalid newline")
	}

	attachments, err := prepareAttachments(msg.Attachments)
	if err != nil {
		return smtpMessage{}, err
	}

	// When the caller explicitly overrides From with a different mailbox than
	// the configured/authenticated identity, surface the latter as Sender so
	// recipients (and SPF/DKIM checks) can tell the two apart.
	senderHeader := ""
	if explicitFrom != "" {
		if defaultTrimmed := strings.TrimSpace(defaultFrom); defaultTrimmed != "" {
			if senderAddr, perr := mail.ParseAddress(defaultTrimmed); perr == nil &&
				!strings.EqualFold(senderAddr.Address, fromAddr.Address) {
				senderHeader = senderAddr.String()
			}
		}
	}

	messageID, err := generateMessageID(fromAddr.Address)
	if err != nil {
		return smtpMessage{}, err
	}

	return smtpMessage{
		envelope: smtpEnvelope{
			from:       fromAddr.Address,
			recipients: recipients,
		},
		fromHeader:   fromAddr.String(),
		senderHeader: senderHeader,
		toHeader:     toHeader,
		ccHeader:     ccHeader,
		replyTo:      replyToHeader,
		subject:      msg.Subject,
		date:         time.Now().Format(time.RFC1123Z),
		messageID:    messageID,
		text:         msg.Text,
		html:         msg.HTML,
		attachments:  attachments,
	}, nil
}

func generateMessageID(fromAddress string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate message id: %w", err)
	}
	domain := "localhost"
	if at := strings.LastIndex(fromAddress, "@"); at >= 0 && at+1 < len(fromAddress) {
		if d := strings.TrimSpace(fromAddress[at+1:]); d != "" {
			domain = d
		}
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(buf[:]), domain), nil
}

func buildMessage(defaultFrom string, msg Message) (smtpEnvelope, []byte, error) {
	message, err := prepareMessage(defaultFrom, msg)
	if err != nil {
		return smtpEnvelope{}, nil, err
	}

	var data bytes.Buffer
	if err := writeMessage(&data, message); err != nil {
		return smtpEnvelope{}, nil, err
	}

	return message.envelope, data.Bytes(), nil
}

func prepareAttachments(values []Attachment) ([]Attachment, error) {
	if len(values) == 0 {
		return nil, nil
	}

	attachments := make([]Attachment, 0, len(values))
	for index, attachment := range values {
		filename := strings.TrimSpace(attachment.Filename)
		if filename == "" {
			return nil, fmt.Errorf("email attachment %d filename is required", index+1)
		}
		if hasHeaderBreak(filename) {
			return nil, fmt.Errorf("email attachment %q filename contains invalid newline", filename)
		}
		contentType := strings.TrimSpace(attachment.ContentType)
		if contentType == "" {
			contentType = defaultAttachmentContentType
		}
		if hasHeaderBreak(contentType) {
			return nil, fmt.Errorf("email attachment %q content type contains invalid newline", filename)
		}
		if _, _, err := mime.ParseMediaType(contentType); err != nil {
			return nil, fmt.Errorf("invalid email attachment %q content type: %w", filename, err)
		}
		if attachment.Content == nil {
			return nil, fmt.Errorf("email attachment %q content reader is required", filename)
		}
		attachments = append(attachments, Attachment{
			Filename:    filename,
			ContentType: contentType,
			Content:     attachment.Content,
		})
	}
	return attachments, nil
}

func parseSingleAddress(fieldName, raw string) (*mail.Address, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("email %s address is required", fieldName)
	}
	if hasHeaderBreak(raw) {
		return nil, fmt.Errorf("email %s address contains invalid newline", fieldName)
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid email %s address: %w", fieldName, err)
	}
	return addr, nil
}

func parseAddressList(fieldName string, values []string) (string, []string, error) {
	formatted := make([]string, 0)
	recipients := make([]string, 0)
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if hasHeaderBreak(raw) {
			return "", nil, fmt.Errorf("email %s address contains invalid newline", fieldName)
		}
		addresses, err := mail.ParseAddressList(raw)
		if err != nil {
			return "", nil, fmt.Errorf("invalid email %s address: %w", fieldName, err)
		}
		for _, addr := range addresses {
			formatted = append(formatted, addr.String())
			recipients = append(recipients, addr.Address)
		}
	}
	return strings.Join(formatted, ", "), recipients, nil
}

func hasHeaderBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func writeMessage(w io.Writer, msg smtpMessage) error {
	if err := writeHeader(w, "From", msg.fromHeader); err != nil {
		return err
	}
	if err := writeHeader(w, "Sender", msg.senderHeader); err != nil {
		return err
	}
	if err := writeHeader(w, "To", msg.toHeader); err != nil {
		return err
	}
	if err := writeHeader(w, "Cc", msg.ccHeader); err != nil {
		return err
	}
	if err := writeHeader(w, "Reply-To", msg.replyTo); err != nil {
		return err
	}
	if err := writeHeader(w, "Subject", mime.QEncoding.Encode("utf-8", msg.subject)); err != nil {
		return err
	}
	if err := writeHeader(w, "Date", msg.date); err != nil {
		return err
	}
	if err := writeHeader(w, "Message-ID", msg.messageID); err != nil {
		return err
	}
	if err := writeHeader(w, "MIME-Version", "1.0"); err != nil {
		return err
	}
	if len(msg.attachments) > 0 {
		return writeMultipartMixed(w, msg)
	}
	return writeBodyHeadersAndContent(w, msg.text, msg.html)
}

func writeHeader(w io.Writer, key, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	_, err := io.WriteString(w, key+": "+value+"\r\n")
	return err
}

func writeBodyHeadersAndContent(w io.Writer, textBody, htmlBody string) error {
	hasText := strings.TrimSpace(textBody) != ""
	hasHTML := strings.TrimSpace(htmlBody) != ""
	if hasText && hasHTML {
		writer := multipart.NewWriter(w)
		if err := writeHeader(w, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", writer.Boundary())); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\r\n"); err != nil {
			return err
		}
		if err := writePart(writer, "text/plain; charset=utf-8", textBody); err != nil {
			return err
		}
		if err := writePart(writer, "text/html; charset=utf-8", htmlBody); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("close multipart body: %w", err)
		}
		return nil
	}

	contentType := "text/plain; charset=utf-8"
	body := textBody
	if hasHTML {
		contentType = "text/html; charset=utf-8"
		body = htmlBody
	}
	if err := writeHeader(w, "Content-Type", contentType); err != nil {
		return err
	}
	if err := writeHeader(w, "Content-Transfer-Encoding", "quoted-printable"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}
	return writeQuotedPrintable(w, body)
}

func writeMultipartMixed(w io.Writer, msg smtpMessage) error {
	writer := multipart.NewWriter(w)
	if err := writeHeader(w, "Content-Type", fmt.Sprintf("multipart/mixed; boundary=%q", writer.Boundary())); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}
	if strings.TrimSpace(msg.text) != "" || strings.TrimSpace(msg.html) != "" {
		if err := writeBodyMultipartPart(writer, msg.text, msg.html); err != nil {
			return err
		}
	}
	for _, attachment := range msg.attachments {
		if err := writeAttachmentPart(writer, attachment); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart mixed body: %w", err)
	}
	return nil
}

func writeBodyMultipartPart(writer *multipart.Writer, textBody, htmlBody string) error {
	hasText := strings.TrimSpace(textBody) != ""
	hasHTML := strings.TrimSpace(htmlBody) != ""
	if hasText && hasHTML {
		var body bytes.Buffer
		alternative := multipart.NewWriter(&body)
		if err := writePart(alternative, "text/plain; charset=utf-8", textBody); err != nil {
			return err
		}
		if err := writePart(alternative, "text/html; charset=utf-8", htmlBody); err != nil {
			return err
		}
		if err := alternative.Close(); err != nil {
			return fmt.Errorf("close multipart alternative body: %w", err)
		}

		header := textproto.MIMEHeader{}
		header.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", alternative.Boundary()))
		part, err := writer.CreatePart(header)
		if err != nil {
			return fmt.Errorf("create multipart body part: %w", err)
		}
		_, err = part.Write(body.Bytes())
		return err
	}

	contentType := "text/plain; charset=utf-8"
	body := textBody
	if hasHTML {
		contentType = "text/html; charset=utf-8"
		body = htmlBody
	}
	return writePart(writer, contentType, body)
}

func writeAttachmentPart(writer *multipart.Writer, attachment Attachment) error {
	contentType, err := attachmentContentType(attachment)
	if err != nil {
		return err
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}))
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create attachment part %q: %w", attachment.Filename, err)
	}

	lineWriter := &base64LineWriter{w: part}
	encoder := base64.NewEncoder(base64.StdEncoding, lineWriter)
	if _, err := io.Copy(encoder, attachment.Content); err != nil {
		encoder.Close()
		return fmt.Errorf("copy attachment %q: %w", attachment.Filename, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encode attachment %q: %w", attachment.Filename, err)
	}
	if err := lineWriter.Close(); err != nil {
		return fmt.Errorf("write attachment %q: %w", attachment.Filename, err)
	}
	return nil
}

func attachmentContentType(attachment Attachment) (string, error) {
	mediaType, params, err := mime.ParseMediaType(attachment.ContentType)
	if err != nil {
		return "", fmt.Errorf("invalid email attachment %q content type: %w", attachment.Filename, err)
	}
	params["name"] = attachment.Filename
	return mime.FormatMediaType(mediaType, params), nil
}

func writePart(writer *multipart.Writer, contentType, body string) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create multipart part: %w", err)
	}
	return writeQuotedPrintable(part, body)
}

func writeQuotedPrintable(w io.Writer, body string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := io.WriteString(qp, body); err != nil {
		qp.Close()
		return err
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("write quoted-printable body: %w", err)
	}
	return nil
}

type base64LineWriter struct {
	w       io.Writer
	lineLen int
}

func (w *base64LineWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		if w.lineLen == base64LineLength {
			if _, err := io.WriteString(w.w, "\r\n"); err != nil {
				return written, err
			}
			w.lineLen = 0
		}
		remaining := base64LineLength - w.lineLen
		if remaining > len(p) {
			remaining = len(p)
		}
		n, err := w.w.Write(p[:remaining])
		written += n
		w.lineLen += n
		p = p[n:]
		if err != nil {
			return written, err
		}
		if n != remaining {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

func (w *base64LineWriter) Close() error {
	if w.lineLen == 0 {
		return nil
	}
	_, err := io.WriteString(w.w, "\r\n")
	w.lineLen = 0
	return err
}
