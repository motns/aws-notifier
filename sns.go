package main

import (
	"encoding/json"
	"strings"
	"errors"
)

/**
Example SNS payload:

{
  "Records": [
    {
      "EventVersion": "1.0",
      "EventSubscriptionArn": "eventsubscriptionarn",
      "EventSource": "aws:sns",
      "Sns": {
        "SignatureVersion": "1",
        "Timestamp": "1970-01-01T00:00:00.000Z",
        "Signature": "EXAMPLE",
        "SigningCertUrl": "EXAMPLE",
        "MessageId": "95df01b4-ee98-5cb9-9903-4c221d41eb5e",
        "Message": "Hello from SNS!",
        "MessageAttributes": {
          "Test": {
            "Type": "String",
            "Value": "TestString"
          },
          "TestBinary": {
            "Type": "Binary",
            "Value": "TestBinary"
          }
        },
        "Type": "Notification",
        "UnsubscribeUrl": "EXAMPLE",
        "TopicArn": "topicarn",
        "Subject": "TestInvoke"
      }
    }
  ]
}

Example Cloudwatch Alarm payload (encoded in Message):

{
	"AlarmName": "Example alarm name",
	"AlarmDescription": "Example alarm description.",
	"AWSAccountId": "000000000000",
	"NewStateValue": "ALARM",
	"NewStateReason": "Threshold Crossed: 1 datapoint (10.0) was greater than or equal to the threshold (1.0).",
	"StateChangeTime": "2017-01-12T16:30:42.236+0000",
	"Region": "EU - Ireland",
	"OldStateValue": "OK",
	"Trigger": {
		"MetricName": "DeliveryErrors",
		"Namespace": "ExampleNamespace",
		"Statistic": "SUM",
		"Unit": null,
		"Dimensions": [],
		"Period": 300,
		"EvaluationPeriods": 1,
		"ComparisonOperator": "GreaterThanOrEqualToThreshold",
		"Threshold": 1.0
	}
}
*/


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Types for reading SNS payloads

type SNSRecordList struct {
	Records []SNSRecord `json:"Records"`
}

type SNSRecord struct {
	EventVersion string `json:"EventVersion"`
	EventSubscriptionArn string `json:"EventSubscriptionArn"`
	EventSource string `json:"EventSource"`
	Sns SNSMessage `json:"Sns"`
}

type SNSMessage struct {
	SignatureVersion string `json:"SignatureVersion"`
	Timestamp string `json:"Timestamp"`
	Signature string `json:"Signature"`
	SigningCertUrl string `json:"SigningCertUrl"`
	MessageId string `json:"MessageId"`
	Message string `json:"Message"`
	MessageAttributes map[string]SMSMessageAttribute `json:"MessageAttributes"`
	Type string `json:"Type"`
	UnsubscribeUrl string `json:"UnsubscribeUrl"`
	TopicArn string `json:"TopicArn"`
	Subject string `json:"Subject"`
}

type SMSMessageAttribute struct {
	Type string `json:"Type"`
	Value string `json:"Value"`
}

type CloudwatchAlarm struct {
	AlarmName string `json:"AlarmName"`
	AlarmDescription string `json:"AlarmDescription"`
	AWSAccountId string `json:"AWSAccountId"`
	NewStateValue string `json:"NewStateValue"`
	NewStateReason string `json:"NewStateReason"`
	StateChangeTime string `json:"StateChangeTime"`
	Region string `json:"Region"`
	OldStateValue string `json:"OldStateValue"`
	Trigger CloudwatchAlarmTrigger `json:"Trigger"`
}

type CloudwatchAlarmTrigger struct {
	MetricName string `json:"MetricName"`
	Namespace string `json:"Namespace"`
	Statistic string `json:"Statistic"`
	Unit string `json:"Unit,omitempty"`
	Dimensions []CloudwatchAlarmTriggerDimension
	Period int `json:"Period"`
	EvaluationPeriods int `json:"EvaluationPeriods"`
	ComparisonOperator string `json:"ComparisonOperator"`
	Threshold float32 `json:"Threshold"`
}

type CloudwatchAlarmTriggerDimension struct {
	Name string `json:"name"`
	Value string `json:"value"`
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Event processor

func processSNSRecords(slackNotifier *SlackNotifier, pagerdutyNotifier *PagerdutyNotifier, raw []byte) error {
	var recordList SNSRecordList

	err := json.Unmarshal(raw, &recordList)
	if err != nil {
		return errors.New("could not unmarshal SNS record list: " + err.Error())
	}

	for _, record := range recordList.Records {
		err := processSNSRecord(slackNotifier, pagerdutyNotifier, record)

		if err != nil {
			return errors.New("could not process SNS record: " + err.Error())
		}
	}

	return nil
}

func processSNSRecord(slackNotifier *SlackNotifier, pagerdutyNotifier *PagerdutyNotifier, record SNSRecord) error {
	// Cloudwatch Alarm
	if strings.Contains(record.Sns.Subject, "ALARM:") || strings.Contains(record.Sns.Subject, "OK:") {
		var alarm CloudwatchAlarm
		isFailing := strings.Contains(record.Sns.Subject, "ALARM:")

		err := json.Unmarshal([]byte(record.Sns.Message), &alarm)
		if err != nil {
			return errors.New("could not unmarshal Cloudwatch Alarm payload: " + err.Error())
		}

		fields := []SlackField {
			{
				Title: record.Sns.Subject,
				Value: alarm.NewStateReason,
				Short: false,
			},
		}

		for _, d := range alarm.Trigger.Dimensions {
			fields = append(fields, SlackField {
				Title: d.Name,
				Value: d.Value,
				Short: true,
			})
		}

		var color string
		if isFailing {
			color = ColorError
		} else {
			color = ColorSuccess
		}

		fields = append(fields, SlackField {
			Title: "Namespace",
			Value: alarm.Trigger.Namespace,
			Short: true,
		})

		fields = append(fields, SlackField {
			Title: "MetricName",
			Value: alarm.Trigger.MetricName,
			Short: true,
		})

		slackMessage := SlackMessage {
			Attachments: []SlackAttachment {
				{
					Fallback: alarm.NewStateReason,
					Color: color,
					Fields: fields,
				},
			},
		}

		if err := slackNotifier.sendMessage(slackMessage); err != nil {
			return err
		}

		if isFailing {
			incidentKey := "incident"
			detailFields := make(map[string]string)

			for _, dv := range alarm.Trigger.Dimensions {
				incidentKey += dv.Value
				detailFields[dv.Name] = dv.Value
			}

			incident := PagerdutyIncident {
				Description: record.Sns.Subject + "-" + alarm.NewStateReason,
				IncidentKey: incidentKey,
				Details: PagerdutyIncidentDetails{
					Fields: detailFields,
				},
			}

			if err := pagerdutyNotifier.triggerIncident(incident); err != nil {
				return err
			}

			return nil
		} else {
			return nil
		}
	} else if strings.Contains(record.Sns.Subject, "RDS Notification Message") {
		// Treat as plain message for now
		// TODO - Implement proper handling (need to work out structure)
		slackMessage := SlackMessage {
			Attachments: []SlackAttachment {
				{
					Fallback:record.Sns.Message,
					Color: ColorInfo,
					Fields: []SlackField {
						{
							Title: record.Sns.Subject,
							Value: record.Sns.Message,
							Short: false,
						},
					},
				},
			},
		}

		if err := slackNotifier.sendMessage(slackMessage); err != nil {
			return err
		}

		return nil
	} else {
		// Basic processing for all other (plain) SNS messages
		slackMessage := SlackMessage {
			Attachments: []SlackAttachment {
				{
					Fallback:record.Sns.Message,
					Color: ColorInfo,
					Fields: []SlackField {
						{
							Title: record.Sns.Subject,
							Value: record.Sns.Message,
							Short: false,
						},
					},
				},
			},
		}

		if err := slackNotifier.sendMessage(slackMessage); err != nil {
			return err
		}

		return nil
	}
}