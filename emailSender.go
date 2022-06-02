package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"regexp"

	"gopkg.in/gomail.v2"
)

type emailSender struct {
	From         string
	mailClient   *gomail.Dialer
	validationRE string
}

func NewEmailSender(host string, port int, from string) *emailSender {
	return &emailSender{
		From:         from,
		mailClient:   &gomail.Dialer{Host: host, Port: port, TLSConfig: &tls.Config{InsecureSkipVerify: true}},
		validationRE: "^[a-zA-Z0-9.!#$%&’*+/=?^_`{|}~-]+@[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*$",
	}
}

func (e *emailSender) Send(dest, subject, message string) error {
	if dest == "" {
		return fmt.Errorf("No destination address given")
	}

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

func (r *emailSender) Validate(address string) bool {
	match, _ := regexp.MatchString(r.validationRE, address)
	return match
}

func (r *emailSender) GetValidationRE() string {
	return r.validationRE
}
