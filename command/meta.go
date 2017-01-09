// deputize - Update an LDAP group with info from the PagerDuty API
// meta.go: subcommand inheritance struct
//
// Copyright 2017 Threat Stack, Inc. All rights reserved.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package command

import "github.com/mitchellh/cli"

// Meta contain the meta-option that nearly all subcommand inherits.
type Meta struct {
	UI cli.Ui
}
