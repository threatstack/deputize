# deputize

`deputize` is a handy tool to update an LDAP group with on-call information
from a PagerDuty schedule.

## Installation

To install, use `go get`: `go get -d github.com/threatstack/deputize`

### Pre-Requisites

Deputize requires an LDAP server that supports StartTLS over port 389. This
LDAP server should have a user that can modify the memberUid attribute of a
group.

Deputize also requires Vault to store secrets.

### Configuration

Deputize is configured using a `config.json` file located in the same directory
as the command (or you can specify a direct path to it using `DEPUTIZE_CONFIG`).
That config file should contain:

```
{
  "BaseDN": "",
  "GrayLogEnabled": "",
  "GrayLogAddress": "",
  "LDAPServer": "",
  "LDAPPort": 0,
  "MailAttribute": "",
  "MemberAttribute": "",
  "ModUserDN": "",
  "OnCallGroup": "",
  "OnCallGroupDN": "",
  "OnCallSchedules": [""],
  "RootCAFile": "",
  "RunDuration": "",
  "SlackChan": "",
  "SlackEnabled": true,
  "TokenPath": "",
  "UserAttribute": "",
  "VaultSecretPath": "",
  "VaultServer": "",
  "Quiet": true
}
```

| Variable          | Type   | Purpose                                                    | Possible Value                        |
|-------------------|--------|------------------------------------------------------------|---------------------------------------|
| `BaseDN`          | String | Base DN for your LDAP server                               | `dc=spiffy,dc=io`                     |
| `GrayLogEnabled`  | String | Enable logging to a GrayLog Server                         | `true`                                |
| `GrayLogAddress`  | String | Graylog Server Address (uses UDP)                          | `graylog.spiffy.io:12201`             |
| `LDAPServer`      | String | Hostname of your LDAP server                               | `ldap.spiffy.io`                      |
| `LDAPPort`        | Int    | Port to talk to LDAP on                                    | `389`                                 |
| `MailAttribute`   | String | LDAP Attribute for a user's email address                  | `mail`                                |
| `MemberAttribute` | String | LDAP Attribute for a group member                          | `memberUid`                           |
| `ModUserDN`       | String | The DN of the user that edits LDAP                         | `cn=deputize,dc=spiffy,dc=io`         |
| `OnCallGroup`     | String | The search string for the LDAP On Call Group               | `(cn=oncall)`                         |
| `OnCallGroupDN`   | String | Full DN for the LDAP On Call Group                         | `cn=oncall,ou=groups,dc=spiffy,dc=io` |
| `OnCallSchedules` | Array  | The names of the PagerDuty Schedules to sync               | `["OnCall1", "OnCall2"]`              |
| `RootCAFile`      | String | A path to a file full of trusted root CAs [See note 1]     | `/etc/ssl/certs/ca-certificates.crt`  |
| `RunDuration`     | String | How far ahead should Deputize look at the oncall schedule? | `1m`                                  |
| `SlackChan`       | Array  | The channel(s) to post update notifications to             | `#security`                           |
| `SlackEnabled`    | Bool   | Do you want Deputize to notify slack?                      | `true`                                |
| `TokenPath`       | String | Path to a file containing a vault token [See note 2]       | `/ramdisk/vault-token`                |
| `UserAttribute`   | String | LDAP Attribute for a User                                  | `uid`                                 |
| `VaultSecretPath` | String | Path to where Vault stores secret information for Deputize | `secret/deputize`                     |
| `VaultServer`     | String | Full path to Vault server                                  | `https://vault.spiffy.io:8200`        |
| `Quiet`           | Bool   | If true, wont display any log output                       | `true`                                |

#### Notes

1. If blank, Go will attempt to use system trust roots.
1. If blank, will attempt to use the `VAULT_TOKEN` environment variable

### LDAP Configuration

There are many LDAP servers in the world, so we can't give a guide to creating
scoped users for all of them. That said, For OpenLDAP, here's a sample
`olcAccess` ACL entry you could use to let a named user edit the `memberUid`
attribute of a specific `posixGroup` entry:
```
olcAccess: to dn.base="cn=oncall,ou=groups,dc=spiffy,dc=io"
  attrs=memberUid
  by dn.exact="cn=deputize,dc=spiffy,dc=io" write
  by * read
```

### Vault Configuration

The location specified for `VaultSecretPath` will need values for:
* `modUserPW`: The password for a user with permission to modify the `memberUid`
  attributes of the oncall group
* `pdAuthToken`: PagerDuty API key
* `slackAuthToken`: Slack API key

## Usage

`deputize oncall` will add the proper oncall rotation, basing it off of
what PagerDuty has scheduled.

Future plans include adding a `deputize me` command for temporary authenticated
access.

## Contribution

1. Fork
1. Create a feature branch
1. Commit your changes
1. Rebase your local changes against the master branch
1. Create a new Pull Request

## Author

Patrick Cable (@patcable)
