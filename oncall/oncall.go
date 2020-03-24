// deputize - Update an LDAP group with info from the PagerDuty API
// oncall.go: On Call Updater
//
// Copyright 2017-2020 Threat Stack, Inc. All rights reserved.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>
// Author: Michael Chmielewski <michael.chmielewski@threatstack.com>

package oncall

import (
  "crypto/tls"
  "crypto/x509"
  "fmt"
  vault "github.com/hashicorp/vault/api"
  "github.com/nlopes/slack"
  "github.com/PagerDuty/go-pagerduty"
  "github.com/xanzy/go-gitlab"
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
  var newOnCallApproverEmails []string
  var newOnCallApproverGitlabUserIds []int

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
            if conf.GitlabEnabled && p.Name == conf.GitlabApproverSchedule {
              newOnCallApproverEmails = append(newOnCallApproverEmails, person.Email)
            }
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

  if conf.GitlabEnabled {
    if !conf.Quiet {
      log.Printf("Gitlab integration enabled. Gitlab group: %s", conf.GitlabGroup)
    }

    gitlabClient := gitlab.NewClient(nil, secret.Data["gitlabAuthToken"].(string))
    gitlabClient.SetBaseURL(conf.GitlabServer+"api/v4")

    // Lets get user ids for On Call people
    for _, email := range newOnCallApproverEmails {
      userOptions := &gitlab.ListUsersOptions{Search: gitlab.String(email)}
      users, _, err := gitlabClient.Users.ListUsers(userOptions)
      if err != nil {
        log.Printf("Warning: Got %s back from Gitlab API\n", err)
      } 
      if len(users) == 1 {
        // We expect only one user returned based on an email. We error out otherwise
        if !conf.Quiet {
          log.Printf("User found! username is %s for email %s\n", users[0].Username, email)
        }
        newOnCallApproverGitlabUserIds = append(newOnCallApproverGitlabUserIds, users[0].ID)
      } else if len(users) == 0 {
        if !conf.Quiet {
          log.Printf("No user found for email %s\n", users[0].Username, email)
        }        
      } else {
        // Lets output some helpful information if we don't get 1 user
        if !conf.Quiet {
          for _, user := range users {
            log.Printf("Found the following users associated with \"%s\": %s\n", email, user.Username)
          }
        }
        return fmt.Errorf("Found more than one user with an email of %s: %d users found", email, len(users))
      }
    }

    if len(newOnCallApproverGitlabUserIds) == 0 {
      // If no users are in the new approver list, leave the group alone
      log.Printf("No new Approvers, not updating Gitlab group: %s", conf.GitlabGroup)
    } else {
      // Add OnCall approvers to approver group
      if !conf.Quiet {
        log.Printf("Updating Gitlab group: %s", conf.GitlabGroup)
      }

      // Remove existing members of the group, if they exist
      if !conf.Quiet {
        log.Printf("Removing old approvers from Gitlab group: %s", conf.GitlabGroup)
      }
      // Get the existing members of the group
      approverGroupMembers, _, err := gitlabClient.Groups.ListGroupMembers(conf.GitlabGroup, &gitlab.ListGroupMembersOptions{})
      if err != nil {
        fmt.Errorf("Gitlab could not get group members: %s\n", err)
      } 
      if len(approverGroupMembers) > 0 {
        // Remove existing members
        for _, member := range approverGroupMembers {
          // Don't remove group owner/maintainers
          if member.AccessLevel < 40 {
            if !conf.Quiet {
              log.Printf("Removing user %s", member.Username)
            }
            _, err := gitlabClient.GroupMembers.RemoveGroupMember(conf.GitlabGroup, member.ID)
            if err != nil {
              fmt.Errorf("Gitlab could not remove group member: %s\n", err)
            }
          }
        }
      }

      // Add new members to the group
      if !conf.Quiet {
        log.Printf("Adding new approvers to Gitlab group: %s", conf.GitlabGroup)
      }
      for _, newApproverUserId := range newOnCallApproverGitlabUserIds {
        if !conf.Quiet {
          log.Printf("Adding user id %d", newApproverUserId)
        }
        addGroupMemberOpts := &gitlab.AddGroupMemberOptions{
                                UserID: gitlab.Int(newApproverUserId),
                                AccessLevel: gitlab.AccessLevel(gitlab.DeveloperPermissions),
        }
        _, _, err := gitlabClient.GroupMembers.AddGroupMember(conf.GitlabGroup, addGroupMemberOpts)
        if err != nil {
          fmt.Errorf("Gitlab could not add group member: %s\n", err)
        } 
      }
    }
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
