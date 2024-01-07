package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// config.go - config functions
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

type deputizeConfig struct {
	SecretPath   string
	SecretRegion string
	Source       deputizeSourceConfig
	Sinks        deputizeSinkConfig
}

type deputizeSourceConfig struct {
	PagerDuty deputizePDConfig
}

type deputizeSinkConfig struct {
	Gitlab deputizeGitlabConfig
	LDAP   deputizeLDAPConfig
	Slack  deputizeSlackConfig
}

type deputizeGitlabConfig struct {
	ApproverSchedule string
	Enabled          bool
	Group            string
	Server           string
}

type deputizeLDAPConfig struct {
	Enabled            bool
	BaseDN             string
	RootCAFile         string
	Server             string
	Port               int
	MailAttribute      string
	MemberAttribute    string
	ModUserDN          string
	OnCallGroup        string
	UserAttribute      string
	InsecureSkipVerify bool
}

type deputizePDConfig struct {
	Enabled         bool
	OnCallSchedules []string
}

type deputizeSlackConfig struct {
	Channels    []string
	Enabled     bool
	PostMessage bool
}

type deputizeSecrets struct {
	GitlabAuthToken     string
	LDAPModUserPassword string
	PDAuthToken         string
	SlackAuthToken      string
}

func validateConfig(cfg *deputizeConfig) error {
	var configErrors []string

	if cfg.SecretPath == "" {
		configErrors = append(configErrors, "SecretPath not set")
	}
	if cfg.SecretRegion == "" {
		cfg.SecretRegion = os.Getenv("AWS_REGION")
	}

	// Sources
	if !cfg.Source.PagerDuty.Enabled {
		configErrors = append(configErrors, "Source: No source enabled")
	}

	if cfg.Source.PagerDuty.Enabled {
		if len(cfg.Source.PagerDuty.OnCallSchedules) == 0 {
			configErrors = append(configErrors, "Pagerduty Source: No On Call Groups Selected")
		}
	}

	// Sinks
	if cfg.Sinks.Gitlab.Enabled {
		if cfg.Sinks.Gitlab.Server == "" {
			configErrors = append(configErrors, "Gitlab Sink: Server not configured")
		}
		if cfg.Sinks.Gitlab.Group == "" {
			configErrors = append(configErrors, "Gitlab Sink: Group not configured")
		}
		if cfg.Sinks.Gitlab.ApproverSchedule == "" {
			configErrors = append(configErrors, "Gitlab Sink: ApproverSchedule not configured")
		}
	}
	if cfg.Sinks.LDAP.Enabled {
		if cfg.Sinks.LDAP.BaseDN == "" {
			configErrors = append(configErrors, "LDAP Sink: BaseDN not configured")
		}
		if cfg.Sinks.LDAP.MailAttribute == "" {
			cfg.Sinks.LDAP.MailAttribute = "mail"
		}
		if cfg.Sinks.LDAP.MemberAttribute == "" {
			cfg.Sinks.LDAP.MemberAttribute = "memberOf"
		}
		if cfg.Sinks.LDAP.ModUserDN == "" {
			configErrors = append(configErrors, "LDAP Sink: ModUserDN not configured")
		}
		if cfg.Sinks.LDAP.OnCallGroup == "" {
			configErrors = append(configErrors, "LDAP Sink: OnCallGroup not configured")
		}
		if cfg.Sinks.LDAP.Port < 1 || cfg.Sinks.LDAP.Port > 65535 {
			configErrors = append(configErrors, "LDAP Sink: Port is invalid")
		}
		if cfg.Sinks.LDAP.RootCAFile == "" {
			cfg.Sinks.LDAP.RootCAFile = "truststore.pem"
		}
		if cfg.Sinks.LDAP.Server == "" {
			configErrors = append(configErrors, "LDAP Sink: Server not configured")
		}
		if cfg.Sinks.LDAP.UserAttribute == "" {
			cfg.Sinks.LDAP.UserAttribute = "uid"
		}
	}
	if cfg.Sinks.Slack.Enabled {
		if len(cfg.Sinks.Slack.Channels) == 0 {
			configErrors = append(configErrors, "Slack Sink: Channels not configured")
		}
	}

	if len(configErrors) > 0 {
		return fmt.Errorf("config validation error(s): %s", buildErrorMsg(configErrors))
	}

	log.Printf("Config: %+v", cfg)
	return nil
}

func buildSecrets(c *deputizeConfig) (deputizeSecrets, error) {
	var configErrors []string

	svcCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(c.SecretRegion))
	if err != nil {
		return deputizeSecrets{}, fmt.Errorf("could not initialize aws svc cfg: %s", err)
	}
	svc := secretsmanager.NewFromConfig(svcCfg)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(c.SecretPath),
	}
	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		return deputizeSecrets{}, fmt.Errorf("could not get secret: %s", err)
	}

	var sec deputizeSecrets
	json.Unmarshal([]byte(*result.SecretString), &sec)

	if c.Source.PagerDuty.Enabled && sec.PDAuthToken == "" {
		configErrors = append(configErrors, "PagerDuty source is enabled, but there's an empty or nonexistant value for PDAuthToken in AWS Secrets Manager")
	}
	if c.Sinks.Gitlab.Enabled && sec.GitlabAuthToken == "" {
		configErrors = append(configErrors, "Gitlab sink is enabled, but there's an empty or nonexistant GitlabAuthToken value in AWS Secrets Manager")
	}
	if c.Sinks.LDAP.Enabled && sec.LDAPModUserPassword == "" {
		configErrors = append(configErrors, "LDAP sink is enabled, but there's an empty or nonexistant LDAPModUserPassword value in AWS Secrets Manager")
	}
	if c.Sinks.Slack.Enabled && sec.SlackAuthToken == "" {
		configErrors = append(configErrors, "Slack sink is enabled, but there's an empty or nonexistant SlackAuthToken value in AWS Secrets Manager")
	}

	if len(configErrors) > 0 {
		return deputizeSecrets{}, fmt.Errorf(buildErrorMsg(configErrors))
	}

	return sec, nil
}

func buildErrorMsg(errs []string) string {
	var niceErrors string
	numErrors := len(errs)
	for i, v := range errs {
		niceErrors = niceErrors + fmt.Sprintf("%d. %s", i+1, v)
		if i+1 < numErrors {
			niceErrors = niceErrors + ", "
		}
	}
	return niceErrors
}
