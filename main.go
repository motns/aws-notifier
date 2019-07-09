package main

import (
	"encoding/json"
	"errors"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"os"
)


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func contains(s []string, el string) bool {
	for _, v := range s {
		if v == el {
			return true
		}
	}

	return false
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// List of supported event sources available here: https://docs.aws.amazon.com/lambda/latest/dg/invoking-lambda-function.html
// List of example event payloads available here: https://docs.aws.amazon.com/lambda/latest/dg/eventsources.html

// Struct with generic type, which we'll use only for detecting what sort of
// Event is being passed to our Lambda
type GenericEvent struct {
	Records []map[string]interface{} `json:"Records,omitempty"`
	Id string `json:"id,omitempty"`
	DetailType string `json:"detail-type,omitempty"`
	Source string `json:"source,omitempty"`
	Test string `json:"test"`
}

func processMessage(slackNotifier *SlackNotifier, pagerdutyNotifier *PagerdutyNotifier, raw json.RawMessage) error {
	var data GenericEvent

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return errors.New("unsupported payload: " + err.Error())
	}

	if data.Records != nil && len(data.Records) != 0 {
		if data.Records[0]["EventSource"] == "aws:sns" {
			err = processSNSRecords(slackNotifier, pagerdutyNotifier, raw)

			if err != nil {
				return err
			}
		} else {
			log.Print("No SNS records to process")
		}
	} else { // Forward everything else to Cloudwatch Event processor (we'll weed unsupported stuff out there)
		err = processCloudwatchEvent(slackNotifier, pagerdutyNotifier, raw)

		if err != nil {
			return err
		}
	}

	return nil
}


///////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////

func HandleRequest(rawData json.RawMessage) error {
	log.Print("Receiving new Event(s)")

	slackWebhook, exists := os.LookupEnv("slack_webhook")
	if !exists {
		return errors.New("could not read slack_webhook_enc from environment")
	}

	slackNotifier := &SlackNotifier{
		webhook: slackWebhook,
	}

	pagerdutyKey, exists := os.LookupEnv("pagerduty_key")
	if !exists {
		return errors.New("could not read pagerduty_key from environment")
	}

	pagerdutyNotifier := &PagerdutyNotifier{
		serviceKey: pagerdutyKey,
	}

	return processMessage(slackNotifier, pagerdutyNotifier, rawData)
}

func main() {
	lambda.Start(HandleRequest)
}
