// deputize.go - main function
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

package main

import (
	"context"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(runLambda)
}

func runLambda(ctx context.Context, cfg *deputizeConfig) (string, error) {

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	err := validateConfig(cfg)
	if err != nil {
		return "", err
	}

	sec, err := buildSecrets(cfg)
	if err != nil {
		return "", err
	}

	oncallEmails, err := getPagerdutyInfo(ctx, cfg.Source.PagerDuty.WithOAuth, sec.PDAuthToken, cfg.Source.PagerDuty.OnCallSchedules)
	if err != nil {
		return "", err
	}

	log.Printf("Current On-Call Users: %s\n", strings.Join(oncallEmails, ", "))

	if cfg.Sinks.LDAP.Enabled {
		err := updateLDAP(cfg.Sinks.LDAP, oncallEmails, sec.LDAPModUserPassword)
		if err != nil {
			return "", err
		}
	}

	if cfg.Sinks.Gitlab.Enabled {
		gitlabApprovers, err := getPagerdutyInfo(ctx, cfg.Source.PagerDuty.WithOAuth, sec.PDAuthToken, []string{cfg.Sinks.Gitlab.ApproverSchedule})
		if err != nil {
			return "", err
		}
		log.Printf("Gitlab Approvers: %s\n", strings.Join(gitlabApprovers, ", "))
		err = updateGitlab(cfg.Sinks.Gitlab, gitlabApprovers, sec.GitlabAuthToken)
		if err != nil {
			return "", err
		}
	}

	if cfg.Sinks.Slack.Enabled {
		err := updateSlack(cfg.Sinks.Slack, oncallEmails, sec.SlackAuthToken)
		if err != nil {
			return "", err
		}
	}

	return strings.Join(oncallEmails, ", "), nil
}
