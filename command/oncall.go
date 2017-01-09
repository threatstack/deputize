// deputize - Update an LDAP group with info from the PagerDuty API
// oncall.go: struct for oncall command
//
// Copyright 2017 Threat Stack, Inc. All rights reserved.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package command

import (
  "log"
  "github.com/threatstack/deputize/oncall"
  "strings"
)

// OncallCommand gets the data from the CLI
type OncallCommand struct {
	Meta
}

// Run actually runs oncall/oncall.go
func (c *OncallCommand) Run(args []string) int {
  err := oncall.UpdateOnCallRotation()
  if err != nil {
    log.Fatalf("Oncall update failed: %s", err)
  }
  return 0
}

// Synopsis gives the help output for oncall
func (c *OncallCommand) Synopsis() string {
	return "Gets oncall schedule from PagerDuty and updates LDAP"
}

// Help prints more useful info for oncall
func (c *OncallCommand) Help() string {
	helpText := `
Usage: deputize oncall

  Pull current oncall schedule from PagerDuty and update LDAP.

  This command will connect to PagerDuty using an API key and pull the
  email addresses of the people who are on call. It'll connect to LDAP
  and pull the members of the lg-oncall group. If there's a difference,
  it will replace the lg-oncall group with the members from PagerDuty.
`
	return strings.TrimSpace(helpText)
}
