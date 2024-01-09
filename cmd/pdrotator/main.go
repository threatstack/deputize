// pdrotator.go - PagerDuty OAuth Rotator Tool for AWS Secrets Manager
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type event struct {
	SecretID           string
	ClientRequestToken string
	Step               string
}

type pagerDutyConfig struct {
	ID     string
	Region string
	Secret string
	Scopes []string
}

type pagerDutyResponse struct {
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

func main() {
	lambda.Start(runLambda)
}

func runLambda(ctx context.Context, e *event) (string, error) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("starting pdrotator from event: %+v\n", e)
	svcCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return "", fmt.Errorf("could not initialize aws svc cfg: %s", err)
	}
	svc := secretsmanager.NewFromConfig(svcCfg)
	input := secretsmanager.DescribeSecretInput{
		SecretId: aws.String(e.SecretID),
	}
	secret, err := svc.DescribeSecret(ctx, &input)
	if err != nil {
		return "", fmt.Errorf("unable to describe secret")
	}
	if !aws.ToBool(secret.RotationEnabled) {
		return "", fmt.Errorf("secret not enabled for rotation")
	}

	_, ok := secret.VersionIdsToStages[e.ClientRequestToken]
	if !ok {
		return "", fmt.Errorf("secret version %s has no stage for rotation of secret %s", e.ClientRequestToken, e.SecretID)
	}

	if contains(secret.VersionIdsToStages[e.ClientRequestToken], "AWSCURRENT") {
		return "", fmt.Errorf("secret version %s already set as AWSCURRENT for secret %s", e.ClientRequestToken, e.SecretID)
	}

	if !contains(secret.VersionIdsToStages[e.ClientRequestToken], "AWSPENDING") {
		return "", fmt.Errorf("secret version %s not set as AWSPENDING for rotation of secret %s", e.ClientRequestToken, e.SecretID)
	}

	switch e.Step {
	case "createSecret":
		err := createSecret(ctx, svc, e)
		if err != nil {
			return "", err
		}
	case "setSecret":
		err := setSecret(ctx, svc, e)
		if err != nil {
			return "", err
		}
	case "testSecret":
		err := testSecret(ctx, svc, e)
		if err != nil {
			return "", err
		}
	case "finishSecret":
		err := finishSecret(ctx, svc, e)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("invalid step: %s", e.Step)
	}

	return "", nil
}

func createSecret(ctx context.Context, svc *secretsmanager.Client, e *event) error {
	// Confirm secret version exists
	getInput := secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(e.SecretID),
		VersionStage: aws.String("AWSCURRENT"),
	}
	currentSecret, gerr := svc.GetSecretValue(ctx, &getInput)
	if gerr != nil {
		return fmt.Errorf("unable to GetSecretValue: %s", gerr)
	}

	// Parse out the instance name from the secret name
	instance := strings.Split(aws.ToString(currentSecret.Name), "/")
	// Get configuration, put it into the struct
	var pdrotator_config_path string
	if os.Getenv("PDROTATOR_CONFIG_PATH") == "" {
		pdrotator_config_path = "deputize/source/pagerduty"
	} else {
		pdrotator_config_path = os.Getenv("PDROTATOR_CONFIG_PATH")
	}
	getPDInput := secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(pdrotator_config_path),
		VersionStage: aws.String("AWSCURRENT"),
	}
	pdConfig, gperr := svc.GetSecretValue(ctx, &getPDInput)
	if gperr != nil {
		return fmt.Errorf("unable to GetSecretValue: %s", gperr)
	}
	var cfg map[string]pagerDutyConfig
	perr := json.Unmarshal([]byte(*pdConfig.SecretString), &cfg)
	if perr != nil {
		return fmt.Errorf("cant unmarshal json from secret manager: %s", perr)
	}

	// Get new PD Token
	token, perr := getPDToken(cfg[instance[len(instance)-1]], instance[len(instance)-1])
	if perr != nil {
		return fmt.Errorf("unable to get PD token: %s", perr)
	}

	// set value
	setInput := secretsmanager.PutSecretValueInput{
		SecretId:           aws.String(e.SecretID),
		ClientRequestToken: aws.String(e.ClientRequestToken),
		SecretString:       aws.String(token.AccessToken),
		VersionStages:      []string{"AWSPENDING"},
	}
	_, serr := svc.PutSecretValue(ctx, &setInput)
	if serr != nil {
		return fmt.Errorf("unable to PutSecretValue: %s", serr)
	}

	return nil
}

// setSecret would update a token somewhere else -- but we're getting it from PD so we're
// just going to return success.
func setSecret(ctx context.Context, svc *secretsmanager.Client, e *event) error {
	return nil
}

// testSecret tests the new API key -- but also, we got this from PD.
func testSecret(ctx context.Context, svc *secretsmanager.Client, e *event) error {
	return nil
}

// finishSecret moves AWSPENDING to AWSCURRENT.
func finishSecret(ctx context.Context, svc *secretsmanager.Client, e *event) error {
	input := secretsmanager.DescribeSecretInput{
		SecretId: aws.String(e.SecretID),
	}
	secret, err := svc.DescribeSecret(ctx, &input)
	if err != nil {
		return fmt.Errorf("unable to describe secret")
	}

	var current_version string
	for k, v := range secret.VersionIdsToStages {
		if contains(v, "AWSCURRENT") {
			if k == e.ClientRequestToken {
				return nil
			}
			current_version = k
		}
	}

	uinput := secretsmanager.UpdateSecretVersionStageInput{
		SecretId:            aws.String(e.SecretID),
		VersionStage:        aws.String("AWSCURRENT"),
		MoveToVersionId:     aws.String(e.ClientRequestToken),
		RemoveFromVersionId: aws.String(current_version),
	}
	_, uerr := svc.UpdateSecretVersionStage(ctx, &uinput)
	if uerr != nil {
		return fmt.Errorf("unable to UpdateSecretVersionStage: %s", uerr)
	}
	return nil
}

func getPDToken(cfg pagerDutyConfig, instance string) (pagerDutyResponse, error) {
	oAuthEndpoint := "https://identity.pagerduty.com/oauth/token"

	params := url.Values{}
	params.Add("grant_type", "client_credentials")
	params.Add("client_id", cfg.ID)
	params.Add("client_secret", cfg.Secret)
	params.Add("scope", fmt.Sprintf("as_account-%s.%s %s", cfg.Region, instance, strings.Join(cfg.Scopes, " ")))

	data := strings.NewReader(params.Encode())
	client := &http.Client{}
	req, err := http.NewRequest("POST", oAuthEndpoint, data)
	if err != nil {
		return pagerDutyResponse{}, fmt.Errorf("unable to POST to PD: %s", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return pagerDutyResponse{}, fmt.Errorf("unable to talk to PD: %s", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pagerDutyResponse{}, fmt.Errorf("unable to read PD body: %s", err)
	}

	var token pagerDutyResponse

	perr := json.Unmarshal(body, &token)
	if perr != nil {
		return pagerDutyResponse{}, fmt.Errorf("unable to parse json: %s", perr)
	}

	return token, nil
}

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
