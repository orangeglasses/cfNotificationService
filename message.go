package main

import "encoding/json"

type messageTarget struct {
	Type        string `json:"type"`
	Environment string `json:"environment,omitempty"`
	Id          string `json:"id"`
}

type messageBody struct {
	Id        string        `json:"id"`
	Subject   string        `json:"subject"`
	Message   string        `json:"message"`
	ExpiresIn string        `json:"validity,omitempty"`
	Target    messageTarget `json:"target"`
}

func (m messageBody) MarshalBinary() ([]byte, error) {
	return json.Marshal(m)
}
