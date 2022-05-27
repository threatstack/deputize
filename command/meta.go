// deputize - Update an LDAP group with info from the PagerDuty API
// meta.go: subcommand inheritance struct
//
// Copyright 2017-2022 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.

package command

import "github.com/mitchellh/cli"

// Meta contain the meta-option that nearly all subcommand inherits.
type Meta struct {
	UI cli.Ui
}
