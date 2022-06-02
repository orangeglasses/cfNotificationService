package main

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"text/template"

	"github.com/wagslane/go-rabbitmq"
)

type rabbitSender struct {
	publisher    *rabbitmq.Publisher
	exchange     string
	template     string
	validationRE string
}

func NewRabbitSender(uri, exchange, template string) *rabbitSender {
	publisher, err := rabbitmq.NewPublisher(uri, rabbitmq.Config{})
	if err != nil {
		log.Fatal(err)
	}

	return &rabbitSender{
		publisher:    publisher,
		exchange:     exchange,
		template:     template,
		validationRE: "^(?:0|(?:\\+|00) ?31 ?)(?:(?:[1-9] ?(?:[0-9] ?){8})|(?:6 ?-? ?[1-9] ?(?:[0-9] ?){7})|(?:[1,2,3,4,5,7,8,9]\\d ?-? ?[1-9] ?(?:[0-9] ?){6})|(?:[1,2,3,4,5,7,8,9]\\d{2} ?-? ?[1-9] ?(?:[0-9] ?){5}))$",
	}
}

func (r *rabbitSender) Send(dest, subject, message string) error {
	if dest == "" {
		return fmt.Errorf("No destination address given")
	}
	log.Printf("sending message to %s. Subject: %v, message: %v\n", dest, subject, message)
	payloadTemplate, err := template.New("msg").Parse(r.template)
	if err != nil {
		return err
	}

	payloadData := struct {
		Destination string
		Subject     string
		Message     string
	}{
		Destination: dest,
		Subject:     subject,
		Message:     message,
	}

	var payload bytes.Buffer
	payloadTemplate.Execute(&payload, payloadData)

	err = r.publisher.Publish(payload.Bytes(), []string{""}, rabbitmq.WithPublishOptionsExchange(r.exchange))
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (r *rabbitSender) Validate(address string) bool {
	match, _ := regexp.MatchString(r.validationRE, address)
	return match
}

func (r *rabbitSender) GetValidationRE() string {
	return r.validationRE
}
