package main

import (
	"net/http"
	"encoding/json"
	"bytes"
	"errors"
	"log"
)

type PagerdutyIncidentDetails struct {
	Fields map[string]string `json:"fields"`
}

type PagerdutyIncident struct {
	Description string `json:"description"`
	IncidentKey string `json:"incident_key"`
	Details PagerdutyIncidentDetails `json:"details"`
}

type PagerdutyIncidentRequest struct {
	ServiceKey string `json:"service_key"`
	EventType string `json:"event_type"`
	Description string `json:"description"`
	IncidentKey string `json:"incident_key"`
	Client string `json:"client"`
	Details PagerdutyIncidentDetails `json:"details"`
}

type PagerdutyNotifier struct {
	serviceKey  string
}

func (p *PagerdutyNotifier) triggerIncident(incident PagerdutyIncident) error {
	log.Print("Triggering Pagerduty incident...")

	req := PagerdutyIncidentRequest {
		ServiceKey: p.serviceKey,
		EventType: "trigger",
		Description: incident.Description,
		IncidentKey: incident.IncidentKey,
		Client: "AWS Event Processor",
		Details: incident.Details,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return errors.New("failed to marshal Pagerduty request: " + err.Error())
	}

	res, err := http.Post(
		"https://events.pagerduty.com/generic/2010-04-15/create_event.json",
		"application/json",
		bytes.NewBuffer(payload))

	res.Body.Close()

	if err != nil {
		return errors.New("failed to trigger Pagerduty Incident - got error: " + err.Error())
	}

	log.Print("Pagerduty incident triggered")

	return nil
}
