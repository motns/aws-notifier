package main

import (
	"net/http"
	"bytes"
	"encoding/json"
	"errors"
	"log"
)

const ColorInfo = "#00BFFF" // Deep Sky Blue
const ColorSuccess = "#00FF00" // Lime
const ColorWarn = "#FFD700" // Gold
const ColorError = "#DC143C" // Crimson

type SlackMessage struct {
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Fallback string `json:"fallback"`
	Color string `json:"color"`
	Fields []SlackField `json:"fields"`
}

type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool `json:"short"`
}

type SlackNotifier struct {
	webhook string
}

func (n *SlackNotifier) sendMessage(msg SlackMessage) error {
	log.Print("Sending Slack message...")

	payload, err := json.Marshal(msg)
	if err != nil {
		return errors.New("Failed to marshal Slack message: " + err.Error())
	}

	res, err := http.Post(n.webhook, "application/json", bytes.NewBuffer(payload))
	res.Body.Close()
	if err != nil {
		return errors.New("Failed to send Slack message - got error: " + err.Error())
	}

	log.Print("Slack message sent")

	return nil
}
