// deputize - Update an LDAP group with info from the PagerDuty API
// main.go: CLI initialization
//
// Copyright 2017 Threat Stack, Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package main

import "os"

func main() {
	os.Exit(Run(os.Args[1:]))
}
