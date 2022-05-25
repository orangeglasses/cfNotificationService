package main

import (
	"bytes"
	"log"
	"text/template"

	"github.com/wagslane/go-rabbitmq"
)

type rabbitSender struct {
	publisher *rabbitmq.Publisher
	exchange  string
	template  string
}

/*type rabbitNotification struct {
	Payload rabbitPayload `json:"payload"`
}

type rabbitPayload struct {
	Kanaal       string `json:"kanaal"`
	P1           string `json:"p1"`
	Meldingtekst string `json:"meldingtekst"`
}*/

func NewRabbitSender(uri, exchange, template string) *rabbitSender {
	publisher, err := rabbitmq.NewPublisher(uri, rabbitmq.Config{})
	if err != nil {
		log.Fatal(err)
	}

	return &rabbitSender{
		publisher: publisher,
		exchange:  exchange,
		template:  template,
	}
}

func (r *rabbitSender) Send(dest, subject, message string) error {
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

	//ROUTING KEY?
	err = r.publisher.Publish(payload.Bytes(), nil, rabbitmq.WithPublishOptionsExchange(r.exchange))
	if err != nil {
		log.Fatal(err)
	}
	return nil
}
