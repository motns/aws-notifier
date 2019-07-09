package main

import (
	"encoding/json"
	"errors"
)

/**
Example EC2 Cloudwatch Event:

{
   "id":"7bf73129-1428-4cd3-a780-95db273d1602",
   "detail-type":"EC2 Instance State-change Notification",
   "source":"aws.ec2",
   "account":"123456789012",
   "time":"2015-11-11T21:29:54Z",
   "region":"us-east-1",
   "resources":[
      "arn:aws:ec2:us-east-1:123456789012:instance/i-abcd1111"
   ],
   "detail":{
      "instance-id":"i-abcd1111",
      "state":"pending"
   }
}

Example Autoscaling Lifecycle Event (launch):

{
  "version": "0",
  "id": "6a7e8feb-b491-4cf7-a9f1-bf3703467718",
  "detail-type": "EC2 Instance-launch Lifecycle Action",
  "source": "aws.autoscaling",
  "account": "123456789012",
  "time": "2015-12-22T18:43:48Z",
  "region": "us-east-1",
  "resources": [
    "arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:59fcbb81-bd02-485d-80ce-563ef5b237bf:autoScalingGroupName/sampleASG"
  ],
  "detail": {
    "LifecycleActionToken": "c613620e-07e2-4ed2-a9e2-ef8258911ade",
    "AutoScalingGroupName": "my-asg",
    "LifecycleHookName": "my-lifecycle-hook",
    "EC2InstanceId": "i-1234567890abcdef0",
    "LifecycleTransition": "autoscaling:EC2_INSTANCE_LAUNCHING",
    "NotificationMetadata": "additional-info"
  }
}


Example Autoscaling EC2 Instance Launch:

{
  "id": "3e3c153a-8339-4e30-8c35-687ebef853fe",
  "detail-type": "EC2 Instance Launch Successful",
  "source": "aws.autoscaling",
  "account": "123456789012",
  "time": "2015-11-11T21:31:47Z",
  "region": "us-east-1",
  "resources": [
		"arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:eb56d16b-bbf0-401d-b893-d5978ed4a025:autoScalingGroupName/ASGLaunchSuccess",
		"arn:aws:ec2:us-east-1:123456789012:instance/i-b188560f"
	],
  "detail": {
		"StatusCode": "InProgress",
		"AutoScalingGroupName": "ASGLaunchSuccess",
		"ActivityId": "9cabb81f-42de-417d-8aa7-ce16bf026590",
		"Details": {
			"Availability Zone": "us-east-1b",
			"Subnet ID": "subnet-95bfcebe"
		},
		"RequestId": "9cabb81f-42de-417d-8aa7-ce16bf026590",
		"EndTime": "2015-11-11T21:31:47.208Z",
		"EC2InstanceId": "i-b188560f",
		"StartTime": "2015-11-11T21:31:13.671Z",
		"Cause": "At 2015-11-11T21:31:10Z a user request created an Auto Scaling group changing the desired capacity from 0 to 1. At 2015-11-11T21:31:11Z an instance was started in response to a difference between desired and actual capacity, increasing the capacity from 0 to 1."
	}
}

*/


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Types for reading Cloudwatch payloads

// Common Event headers - the "detail" part is different for each Event Type
type CloudwatchEvent struct {
	Id string `json:"id"`
	DetailType string `json:"detail-type"`
	Source string `json:"source"`
	Account string `json:"account"`
	Time string `json:"time"`
	Region string `json:"region"`
	Resources []string `json:"resources"`
	Detail json.RawMessage `json:"detail"`
}

type DetailEC2StateChange struct {
	InstanceId string `json:"instance-id"`
	State string `json:"state"`
}

type DetailAutoScalingLifecycleEvent struct {
	LifecycleActionToken string `json:"LifecycleActionToken"`
	AutoScalingGroupName string `json:"AutoScalingGroupName"`
	LifecycleHookName string `json:"LifecycleHookName"`
	EC2InstanceId string `json:"EC2InstanceId"`
	LifecycleTransition string `json:"LifecycleTransition"`
	NotificationMetadata string `json:"NotificationMetadata"`
}

type DetailAutoScalingEC2Event struct {
	StatusCode string `json:"StatusCode"`
	AutoScalingGroupName string `json:"AutoScalingGroupName"`
	ActivityId string `json:"ActivityId"`
	Details DetailAutoScalingEC2EventDetails `json:"Details"`
	RequestId string `json:"RequestId"`
	EndTime string `json:"EndTime"`
	EC2InstanceId string `json:"EC2InstanceId"`
	StartTime string `json:"StartTime"`
	Cause string `json:"Cause"`
}

type DetailAutoScalingEC2EventDetails struct {
	AvailabilityZone string `json:"Availability Zone"`
	SubnetID string `json:"Subnet ID"`
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func processCloudwatchEvent(slackNotifier *SlackNotifier, pagerdutyNotifier *PagerdutyNotifier, raw []byte) error {
	var event CloudwatchEvent

	err := json.Unmarshal(raw, &event)
	if err != nil {
		return errors.New("unsupported Cloudwatch Event payload: " + err.Error())
	}

	// EC2 start/stop notifications
	if event.Source == "aws.ec2" {
		if event.DetailType == "EC2 Instance State-change Notification" {
			err = processEC2StateChangeEvent(slackNotifier, pagerdutyNotifier, event)

			if err != nil {
				return errors.New("failed to process EC2 Event: " + err.Error())
			}
		}
	} else if event.Source == "aws.events" { // Cloudwatch Scheduled Event
		// Ignore for now
		return nil
	} else if event.Source == "aws.autoscaling" {
		err = processAutoscalingEvent(slackNotifier, pagerdutyNotifier, event)

		if err != nil {
			return errors.New("failed to process Autoscaling Event: " + err.Error())
		}
	} else {
		// Generic handler for all other types
		title := event.Source
		slackMessage := SlackMessage {
			Attachments: []SlackAttachment {
				{
					Fallback: title,
					Color: ColorInfo,
					Fields: []SlackField {
						{
							Title: "CloudWatch Event",
							Value: title,
							Short: false,
						},
						{
							Title: "Event Detail JSON",
							Value: string(event.Detail),
							Short: false,
						},
					},
				},
			},
		}

		if err := slackNotifier.sendMessage(slackMessage); err != nil {
			return err
		}
	}

	return nil
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func processEC2StateChangeEvent(slackNotifier *SlackNotifier, pagerdutyNotifier *PagerdutyNotifier, event CloudwatchEvent) error {
	// TODO - Grab instance more info here
	var eventDetail DetailEC2StateChange

	err := json.Unmarshal(event.Detail, &eventDetail)
	if err != nil {
		return errors.New("unsupported EC2 Cloudwatch Event Detail: " + err.Error())
	}

	var color string
	if contains([]string{"shutting-down", "terminated", "stopping", "stopped"}, eventDetail.State) {
		color = ColorWarn
	} else {
		color = ColorInfo
	}

	title := "EC2 Instance State-change"
	slackMessage := SlackMessage {
		Attachments: []SlackAttachment {
			{
				Fallback: title,
				Color: color,
				Fields: []SlackField {
					{
						Title: "CloudWatch Event",
						Value: title,
						Short: false,
					},
					{
						Title: "instance-id",
						Value: eventDetail.InstanceId,
						Short: true,
					},
					{
						Title: "state",
						Value: eventDetail.State,
						Short: true,
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

func processAutoscalingEvent(slackNotifier *SlackNotifier, pagerdutyNotifier *PagerdutyNotifier, event CloudwatchEvent) error {
	var slackMessage SlackMessage

	if contains([]string{"EC2 Instance-launch Lifecycle Action", "EC2 Instance-terminate Lifecycle Action"}, event.DetailType) {
		var eventDetail DetailAutoScalingLifecycleEvent

		title := "Autoscaling - Lifecycle Action"
		slackMessage = SlackMessage {
			Attachments: []SlackAttachment {
				{
					Fallback: title,
					Color: ColorInfo,
					Fields: []SlackField {
						{
							Title: "CloudWatch Event",
							Value: title,
							Short: false,
						},
						{
							Title: "AutoScalingGroupName",
							Value: eventDetail.AutoScalingGroupName,
							Short: true,
						},
						{
							Title: "EC2InstanceId",
							Value: eventDetail.EC2InstanceId,
							Short: true,
						},
						{
							Title: "LifecycleTransition",
							Value: eventDetail.LifecycleTransition,
							Short: true,
						},
					},
				},
			},
		}

		if err := slackNotifier.sendMessage(slackMessage); err != nil {
			return err
		}

	} else {
		var eventDetail DetailAutoScalingEC2Event

		var color string
		if contains([]string{"EC2 Instance Launch Unsuccessful", "EC2 Instance Terminate Unsuccessful"}, event.DetailType) {
			color = ColorWarn
		} else {
			color = ColorInfo
		}

		title := "Autoscaling - " + event.DetailType
		slackMessage = SlackMessage {
			Attachments: []SlackAttachment {
				{
					Fallback: title,
					Color: color,
					Fields: []SlackField {
						{
							Title: "CloudWatch Event",
							Value: title,
							Short: false,
						},
						{
							Title: "EC2InstanceId",
							Value: eventDetail.EC2InstanceId,
							Short: true,
						},
						{
							Title: "StatusCode",
							Value: eventDetail.StatusCode,
							Short: true,
						},
						{
							Title: "Availability Zone",
							Value: eventDetail.Details.AvailabilityZone,
							Short: true,
						},
						{
							Title: "Cause",
							Value: eventDetail.Cause,
							Short: true,
						},
					},
				},
			},
		}

		if err := slackNotifier.sendMessage(slackMessage); err != nil {
			return err
		}
	}

	return nil
}