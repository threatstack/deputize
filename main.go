// deputize - Update an LDAP group with info from the PagerDuty API
// main.go: CLI initialization
//
// Copyright 2017-2022 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.

package main

import "os"

func main() {
	os.Exit(Run(os.Args[1:]))
}
