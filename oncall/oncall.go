// deputize - Update an LDAP group with info from the PagerDuty API
// oncall.go: On Call Updater
//
// Copyright 2017 Threat Stack, Inc. All rights reserved.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package oncall

import (
  "crypto/tls"
  "crypto/x509"
  "fmt"
  vault "github.com/hashicorp/vault/api"
  "github.com/nlopes/slack"
  "github.com/PagerDuty/go-pagerduty"
  "github.com/threatstack/deputize/config"
  "gopkg.in/ldap.v2"
  "io/ioutil"
  "log"
  "os"
  "reflect"
  "strings"
  "time"
)

// UpdateOnCallRotation - read in config and update the on call conf.
func UpdateOnCallRotation(conf config.DeputizeConfig) error {
  // We use vault for storing the LDAP user password, PD token, Slack token
  vaultConfig := vault.DefaultConfig()
  vaultConfig.Address = conf.VaultServer
  vaultClient, err := vault.NewClient(vaultConfig)
  if err != nil {
    return fmt.Errorf("Error initializing Vault client: %s\n", err)
  }
  if conf.TokenPath == "" {
    if os.Getenv("VAULT_TOKEN") == "" {
      return fmt.Errorf("TokenPath isn't set & no VAULT_TOKEN env present")
    }
  } else {
    vaultToken, err := ioutil.ReadFile(conf.TokenPath)
    if err != nil {
      return fmt.Errorf("Unable to read host token from %s", conf.TokenPath)
    }
    vaultClient.SetToken(strings.TrimSpace(string(vaultToken)))
  }
  secret, err := vaultClient.Logical().Read("secret/deputize")
  if err != nil {
    return fmt.Errorf("Unable to read secrets from vault: ", conf.VaultSecretPath)
  }

  // Begin talking to PagerDuty
  client := pagerduty.NewClient(secret.Data["pdAuthToken"].(string))
  if !conf.Quiet {
    log.Printf("Deputize starting. Oncall groups: %s", strings.Join(conf.OnCallSchedules[:],", "))
  }
  var newOnCallEmails []string
  var newOnCallUids []string

  // Cycle through the schedules and once we hit one we care about, get the
  // email address of the person on call for the date period between runtime
  // and runtime+12 hours
  var lsSchedulesOpts pagerduty.ListSchedulesOptions
  if allSchedulesPD, err := client.ListSchedules(lsSchedulesOpts); err != nil {
    return fmt.Errorf("PagerDuty Client says: %s", err)
  } else {
    for _, p := range allSchedulesPD.Schedules {
      if contains(conf.OnCallSchedules, p.Name) {
        // We've hit one of the schedules we care about, so let's get the list
        // of on-call users between today and +12 hours.
        var onCallOpts pagerduty.ListOnCallUsersOptions
        var currentTime = time.Now()
        onCallOpts.Since = currentTime.Format("2006-01-02T15:04:05Z07:00")
        hours, _ := time.ParseDuration(conf.RunDuration)
        onCallOpts.Until = currentTime.Add(hours).Format("2006-01-02T15:04:05Z07:00")
        if !conf.Quiet {
          log.Printf("Getting oncall for schedule \"%s\" (%s) between %s and %s",
            p.Name, p.APIObject.ID, onCallOpts.Since, onCallOpts.Until)
        }
        if oncall, err := client.ListOnCallUsers(p.APIObject.ID, onCallOpts); err != nil {
            return fmt.Errorf("Unable to ListOnCallUsers: %s", err)
        } else {
          for _, person := range oncall {
            newOnCallEmails = append(newOnCallEmails, person.Email)
          }
        }
      }
    }
  }

  // Now to figure out what LDAP user the email correlates to
  l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", conf.LDAPServer, conf.LDAPPort))
  if err != nil {
    log.Fatal(err)
  }
  defer l.Close()

  // RootCA setup
  tlsConfig := &tls.Config{
      InsecureSkipVerify: false,
      ServerName: conf.LDAPServer,
    }

  if conf.RootCAFile != "" {
    rootCerts := x509.NewCertPool()
    rootCAFile, err := ioutil.ReadFile(conf.RootCAFile)
    if err != nil {
      return fmt.Errorf("Unable to read RootCAFile: %s", err)
    }
    if !rootCerts.AppendCertsFromPEM(rootCAFile) {
      return fmt.Errorf("Unable to append to CertPool from RootCAFile")
    }
    tlsConfig.RootCAs = rootCerts
  }

  err = l.StartTLS(tlsConfig)
  if err != nil {
    return fmt.Errorf("Unable to start TLS connection: %s", err)
  }

  // get current members of the oncall group (needed for removal later)
  currentOnCall := search(l, conf.BaseDN, conf.OnCallGroup, []string{conf.MemberAttribute})
  currentOnCallUids := currentOnCall.Entries[0].GetAttributeValues(conf.MemberAttribute)
  if !conf.Quiet {
    log.Printf("Currently on call (LDAP): %s", strings.Join(currentOnCallUids[:],", "))
  }
  // yeah, we *shouldnt* need to do this, but I want to make sure
  // both slices are sorted the same way so DeepEqual works
  currentOnCallUids = removeDuplicates(currentOnCallUids)

  for _, email := range newOnCallEmails {
    newOnCall := search(l, conf.BaseDN, fmt.Sprintf("(%s=%s)", conf.MailAttribute, email), []string{conf.UserAttribute})
    newOnCallUids = append(newOnCallUids, newOnCall.Entries[0].GetAttributeValue("uid"))
  }
  newOnCallUids = removeDuplicates(newOnCallUids)

  if !conf.Quiet {
    log.Printf("New on call (PagerDuty): %s", strings.Join(newOnCallUids[:],", "))
  }
  if reflect.DeepEqual(currentOnCallUids,newOnCallUids) {
    if !conf.Quiet {
      log.Printf("LDAP and PagerDuty match, doing nothing.\n")
    }
  } else {
    if !conf.Quiet {
      log.Printf("Replacing LDAP with PagerDuty information.\n")
    }

    if err := l.Bind(conf.ModUserDN, secret.Data["modUserPW"].(string)); err != nil {
      return fmt.Errorf("Unable to bind to LDAP as %s", conf.ModUserDN)
    }

    if len(currentOnCallUids) > 0 {
      if !conf.Quiet {
        log.Printf("LDAP: Deleting old UIDs")
      }
      delUsers := ldap.NewModifyRequest(conf.OnCallGroupDN)
      delUsers.Delete(conf.MemberAttribute, currentOnCallUids)
      if err = l.Modify(delUsers); err != nil {
        return fmt.Errorf("Unable to delete existing users from LDAP")
      }
    }
    if !conf.Quiet {
      log.Printf("LDAP: Adding new UIDs")
    }
    addUsers := ldap.NewModifyRequest(conf.OnCallGroupDN)
    addUsers.Add(conf.MemberAttribute, newOnCallUids)
    if err = l.Modify(addUsers); err != nil {
      return fmt.Errorf("Unable to add new users to LDAP")
    }

    if conf.SlackEnabled == true {
      slackAPI := slack.New(secret.Data["slackAuthToken"].(string))
      slackParams := slack.PostMessageParameters{}
      slackParams.AsUser = true
      slackMsg := fmt.Sprintf("Updated `%s` on %s: from {%s} to {%s}",
        conf.OnCallGroup,
        conf.LDAPServer,
        strings.Join(currentOnCallUids[:],", "),
        strings.Join(newOnCallUids[:],", "))
      for _, channel := range conf.SlackChan {
        _,_,err := slackAPI.PostMessage(channel, slackMsg, slackParams)
        if err != nil {
          log.Printf("Warning: Got %s back from Slack API\n", err)
        }
      }
    }
  }
  return nil
}
