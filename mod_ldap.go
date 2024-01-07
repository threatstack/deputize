package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"reflect"
	"strings"

	"log"

	"gopkg.in/ldap.v2"
)

func updateLDAP(cfg deputizeLDAPConfig, pdOnCallEmails []string, ldappw string) error {
	log.Printf("Beginning LDAP Update\n")
	client, err := setupLDAPConnection(cfg.Server, cfg.Port, cfg.RootCAFile, cfg.InsecureSkipVerify)
	if err != nil {
		return fmt.Errorf("unable to set up ldap client: %s", err)
	}

	var resolvedLDAPOnCallUIDs []string

	// get current members of the oncall group (needed for removal later)
	currentLDAPOnCall, err := search(client, cfg.BaseDN, fmt.Sprintf("(%s)", cfg.OnCallGroup), []string{cfg.MemberAttribute})
	if err != nil {
		return fmt.Errorf("unable to get current on call from LDAP: %s", err)
	}
	currentLDAPOnCallUIDs := currentLDAPOnCall.Entries[0].GetAttributeValues(cfg.MemberAttribute)
	// yeah, we *shouldnt* need to do this, but I want to make sure
	// both slices are sorted the same way so DeepEqual works
	currentLDAPOnCallUIDs = removeDuplicates(currentLDAPOnCallUIDs)
	log.Printf("Current LDAP OnCall UIDs: %s\n", strings.Join(currentLDAPOnCallUIDs, ","))

	// Resolve the emails from PD to UIDs that we can use to determine if we need to update LDAP
	for _, email := range pdOnCallEmails {
		newOnCall, err := search(client, cfg.BaseDN, fmt.Sprintf("(%s=%s)", cfg.MailAttribute, email), []string{cfg.UserAttribute})
		if err != nil {
			return fmt.Errorf("unable to resolve emails from PD into LDAP UIDs: %s", err)
		}
		resolvedLDAPOnCallUIDs = append(resolvedLDAPOnCallUIDs, newOnCall.Entries[0].GetAttributeValue("uid"))
	}
	resolvedLDAPOnCallUIDs = removeDuplicates(resolvedLDAPOnCallUIDs)
	log.Printf("Resolved New LDAP OnCall UIDs: %s\n", strings.Join(resolvedLDAPOnCallUIDs, ","))

	// Get the DN for the oncall group
	onCallGroup, err := search(client, cfg.BaseDN, fmt.Sprintf("(%s)", cfg.OnCallGroup), []string{"cn"})
	if err != nil {
		return fmt.Errorf("unable to get LDAP OnCall Group DN: %s", err)
	}
	onCallGroupDN := onCallGroup.Entries[0].DN
	log.Printf("On Call Group DN: %s\n", onCallGroupDN)

	// If they're not the same, then theres a difference and we need to update LDAP
	if !reflect.DeepEqual(currentLDAPOnCallUIDs, resolvedLDAPOnCallUIDs) {
		if err := client.Bind(cfg.ModUserDN, ldappw); err != nil {
			return fmt.Errorf("unable to bind to LDAP as %s", cfg.ModUserDN)
		}

		if len(currentLDAPOnCallUIDs) > 0 {
			delUsers := ldap.NewModifyRequest(onCallGroupDN)
			delUsers.Delete(cfg.MemberAttribute, currentLDAPOnCallUIDs)
			if err = client.Modify(delUsers); err != nil {
				return fmt.Errorf("unable to delete existing users from LDAP: %s", err)
			}
			addUsers := ldap.NewModifyRequest(onCallGroupDN)
			addUsers.Add(cfg.MemberAttribute, resolvedLDAPOnCallUIDs)
			if err = client.Modify(addUsers); err != nil {
				return fmt.Errorf("unable to add new users to LDAP: %s", err)
			}
		}
	}
	log.Printf("LDAP Update Complete.\n")
	return nil
}

func setupLDAPConnection(host string, port int, cafile string, insecureSkipVerify bool) (*ldap.Conn, error) {
	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		ServerName:         host,
	}
	rootCerts := x509.NewCertPool()
	rootCAFile, err := os.ReadFile(cafile)
	if err != nil {
		return nil, fmt.Errorf("unable to read trusted CAs from %s: %s", cafile, err)
	}
	if !rootCerts.AppendCertsFromPEM(rootCAFile) {
		return nil, fmt.Errorf("unable to append to trust store: %s", err)
	}
	tlsConfig.RootCAs = rootCerts
	tlsErr := l.StartTLS(tlsConfig)
	if tlsErr != nil {
		return nil, fmt.Errorf("unable to start TLS connection: %s", err)
	}

	return l, nil
}

// This is only good for things you know will return only one result. Be warned.
func search(l *ldap.Conn, basedn string, search string, attributes []string) (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		basedn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		search,
		attributes,
		nil,
	)
	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(sr.Entries) != 1 {
		return nil, fmt.Errorf("user does not exist or too many entries returned")
	}

	return sr, nil
}
