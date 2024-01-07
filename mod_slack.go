// mod_slack.go - Slack sink code
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

package main

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
)

func updateSlack(cfg deputizeSlackConfig, pdOnCallEmails []string, slackAuthToken string) error {
	log.Printf("Beginning Slack Update.\n")
	slackAPI := slack.New(slackAuthToken)
	var oncallUsers []*slack.User
	for _, email := range pdOnCallEmails {
		user, err := slackAPI.GetUserByEmail(email)
		if err != nil {
			return fmt.Errorf("unable to getUserByEmail: %s", err)
		}
		oncallUsers = append(oncallUsers, user)
	}

	var slackUIDs []string
	for _, user := range oncallUsers {
		slackUIDs = append(slackUIDs, user.ID)
	}
	log.Printf("Current Oncall UIDs: %+v\n", slackUIDs)

	for _, channel := range cfg.Channels {
		c, err := slackAPI.GetConversationInfo(&slack.GetConversationInfoInput{ChannelID: channel})
		if err != nil {
			log.Printf("Warning: Got %s back from Slack API\n", err)
		}

		// Does the channel topic have a | in it? that's our delimiter, attempt to split.
		channelTopic := strings.Split(c.Topic.Value, "|")

		// Pull out current On Call folks
		r, _ := regexp.Compile("U[A-Z0-9]+")
		topicUIDs := r.FindAllString(channelTopic[0], -1)

		log.Printf("Oncall UIDs from channel %s: %+v\n", channel, topicUIDs)

		// See if they match w/ current on call, if not then update topic
		if !reflect.DeepEqual(slackUIDs, topicUIDs) {
			log.Printf("Difference between Current and Topic UIDs, updating topic.\n")
			topic := "On-Call: "
			// slackify the UIDs
			for i, uid := range slackUIDs {
				topic = topic + "<@" + uid + ">"
				if i+1 < len(slackUIDs) {
					topic = topic + ", "
				}

			}
			if len(channelTopic) > 1 {
				_, err := slackAPI.SetTopicOfConversation(channel, fmt.Sprintf("%s |%s", topic, channelTopic[1]))
				if err != nil {
					log.Printf("Warning: Got %s back from Slack API\n", err)
				}
			} else {
				_, err := slackAPI.SetTopicOfConversation(channel, fmt.Sprintf("%s |", topic))
				if err != nil {
					log.Printf("Warning: Got %s back from Slack API\n", err)
				}
			}
			if cfg.PostMessage {
				slackParams := slack.PostMessageParameters{}
				slackParams.AsUser = true

				for _, channel := range cfg.Channels {
					_, _, err := slackAPI.PostMessage(channel, slack.MsgOptionPostMessageParameters(slackParams), slack.MsgOptionText(topic, false))
					if err != nil {
						log.Printf("Warning: Got %s back from Slack API\n", err)
					}
				}
			}
		}
	}
	log.Printf("Slack update complete.\n")
	return nil
}
