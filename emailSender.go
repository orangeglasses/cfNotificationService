package main

import (
	"crypto/tls"
	"log"

	"gopkg.in/gomail.v2"
)

type emailSender struct {
	From       string
	mailClient *gomail.Dialer
}

func NewEmailSender(host string, port int, from string) *emailSender {
	return &emailSender{
		From:       from,
		mailClient: &gomail.Dialer{Host: host, Port: port, TLSConfig: &tls.Config{InsecureSkipVerify: true}},
	}
}

func (e *emailSender) Send(dest, subject, message string) error {
	log.Printf("sending message to %s. Subject: %v, message: %v\n", dest, subject, message)

	m := gomail.NewMessage()
	m.SetHeader("From", e.From)
	m.SetHeader("To", dest)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", message)

	if err := e.mailClient.DialAndSend(m); err != nil {
		log.Println("Unable to send mail: ", err)
		return err
	}

	return nil
}
