// deputize - Update an LDAP group with info from the PagerDuty API
// version.go: struct for version command
//
// Copyright 2017-2022 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.

package command

import (
	"bytes"
	"fmt"
)

// VersionCommand stores information about the App version/name
type VersionCommand struct {
	Meta

	Name     string
	Version  string
	Revision string
}

// Run actually execs the version command
func (c *VersionCommand) Run(args []string) int {
	var versionString bytes.Buffer

	fmt.Fprintf(&versionString, "%s version %s", c.Name, c.Version)
	if c.Revision != "" {
		fmt.Fprintf(&versionString, " (%s)", c.Revision)
	}

	c.UI.Output(versionString.String())
	return 0
}

// Synopsis gives an overview of the version command
func (c *VersionCommand) Synopsis() string {
	return fmt.Sprintf("Print %s version and quit", c.Name)
}

// Help would uh, give help output for version.
func (c *VersionCommand) Help() string {
	return ""
}
