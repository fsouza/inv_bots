package lib

import (
	"crypto/tls"
	"io"
	"net/smtp"
	"sync"
)

// GmailSender sends email using Gmail's SMTP server, reusing the underlying
// SMTP connection.
type GmailSender struct {
	conn     *smtp.Client
	user     string
	password string
	mutex    sync.Mutex
}

func NewGmailSender(user, password string) (*GmailSender, error) {
	return &GmailSender{user: user, password: password}, nil
}

func (s *GmailSender) connect() error {
	var err error
	s.conn, err = smtp.Dial("smtp.gmail.com:587")
	if err != nil {
		return err
	}
	err = s.conn.Hello("localhost")
	if err != nil {
		return err
	}
	err = s.conn.StartTLS(&tls.Config{ServerName: "smtp.gmail.com"})
	if err != nil {
		return err
	}
	return s.conn.Auth(smtp.PlainAuth("", s.user, s.password, "smtp.gmail.com"))
}

func (s *GmailSender) SendMail(recipient string, data []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.conn == nil {
		err := s.connect()
		if err != nil {
			return err
		}
	}
	err := s.conn.Mail(s.user)
	if err != nil {
		return err
	}
	err = s.conn.Rcpt(recipient)
	writer, err := s.conn.Data()
	if err != nil {
		return err
	}
	n, err := writer.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return writer.Close()
}

func (s *GmailSender) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.conn != nil {
		err := s.conn.Quit()
		s.conn = nil
		return err
	}
	return nil
}
