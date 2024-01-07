# deputize

Deputize is a tool to read in information from a source (such as PagerDuty) to determine who's on call, and then send that information to a sink (GitLab, LDAP, Slack) to do something with - update an approver group in GitLab, or update an LDAP group, or update a topic/post a message on Slack.

## Prerequisites
If you want to deploy Deputize, you'll need some API keys before you get started.

### Sources
A **Source** is where on-call user information is stored. Deputize can pull the email addresses of on-call folks from PagerDuty. You'll need to:
1. Create a read-only developer API key (https://your-instance-here.pagerduty.com/api_keys)
2. Note the name(s) of the on-call schedule(s) you will be monitoring

### Sinks
A **Sink** is the destination for those on-call emails you read from the source. Deputize supports sending data to the following sinks today:
* GitLab
* LDAP
* Slack

#### GitLab
1. Create an API token for GitLab.
2. Note the URL of your instance (If you're using gitlab.com, set `Server` to `https://gitlab.com/`)
2. Note the path to the approver group (this is used for `Group`)
3. Note the what on-call schedule that will populate that group (this is used for `ApproverSchedule`)

#### LDAP
There are many LDAP servers in the world, so we can't give a guide to creating scoped users for all of them. High level, you'll want to make a user (and set that user as `ModUserDN`) that can modify a named on-call group. For OpenLDAP, here's a sample `olcAccess` ACL entry you could use to let a named user edit the `memberUid` attribute of a specific `posixGroup` entry:
```
olcAccess: to dn.base="cn=oncall,ou=groups,dc=spiffy,dc=io"
  attrs=memberUid
  by dn.exact="cn=deputize,dc=spiffy,dc=io" write
  by * read
```

If you're using a custom CA in your environment, make sure to drop that root CA certificate into `truststore.pem`

#### Slack
Create a new Slack application in your workspace with the following scopes:
* `channels:read`
* `channels:write.topic`
* `chat:write`
* `users:read`
* `users:read.email`

Grab the workspace OAuth Token. It should start with `xoxb-`.

## Deployment

### Create A Secret

Create an AWS Secrets Manager secret, and populate it with the following keys for whichever sources and sinks you're looking to use.

| Key                 | Type     | Purpose
|---------------------|----------|---------------------------------------------------|
| GitlabAuthToken     | Sink   | GitLab API key for updating a GitLab group.         |
| LDAPModUserPassword | Sink   | LDAP password for the user you specify in ModUserDN |
| PDAuthToken         | Source | Read API key for PagerDuty                          |
| SlackAuthToken      | Sink   | Slack Bot Token                                     |


### Create IAM Execution Role
Create an execution IAM role for your Lambda function to execute as. You'll want to give it a policy that allows it to read from AWS Secrets Manager:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "logs:CreateLogGroup",
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Resource": "arn:aws:logs:*:*:*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetResourcePolicy",
                "secretsmanager:GetSecretValue",
                "secretsmanager:DescribeSecret",
                "secretsmanager:ListSecretVersionIds"
            ],
            "Resource": [
                "arn:aws:secretsmanager:us-east-1:ACCOUNT_NUM_HERE:secret:deputize/myEnv-o9iCSb"
            ]
        },
        {
            "Effect": "Allow",
            "Action": "secretsmanager:ListSecrets",
            "Resource": "*"
        }
    ]
}
```

If you're planning on using the LDAP sink and the LDAP server is inside your VPC, you'll want to add the following policy statement:
```
{
    "Sid": "AllowConnectivityToLDAP",
    "Effect": "Allow",
    "Action": [
        "ec2:CreateNetworkInterface",
        "ec2:DescribeNetworkInterfaces",
        "ec2:DeleteNetworkInterface"
    ],
    "Resource": "*"
}
```

### Package up the binary and truststore
You'll need to build a linux amd64 binary with a golang toolchain. Run `GOOS=linux GOARCH=amd64 go build`. 

If you're using the LDAP sink, make sure you've put the root CA for your LDAP server in `truststore.pem`.

Combine the files into a zip file you can upload with `zip deputize.zip deputize truststore.pem`

### Create the Lambda function
1. Specify the IAM execution role you created above
2. Upload the zip file
3. Configure the entrypoint (`deputize`), and RAM settings (128mb is fine)
4. (if using the LDAP sink) Configure the VPC connectivity - subnets and security groups

### Invoke the function
This is where it all comes together. The configuration for Deputize is sent via on invocation. You could set up an EventBridge event to run this function every few minutes.

```
{
  "SecretPath": "deputize/myEnvConfig",
  "SecretRegion": "us-east-1",
  "Source": {
    "PagerDuty": {
      "Enabled": true,
      "OnCallSchedules": ["Ops", "Ops 2nd Level"]
    }
  },
  "Sinks": {
    "Gitlab": {
      "Enabled": false,
      "Server": "https://gitlab.com/",
      "Group": "patcable/approvers",
      "ApproverSchedule": "Ops 2nd Level"
    },
    "LDAP": {
      "Enabled": false,
      "BaseDN": "dc=tls,dc=zone",
      "RootCAFile": "truststore.pem",
      "Server": "ldap.tls.zone",
      "Port": 389,
      "ModUserDN": "cn=deputize,dc=tls,dc=zone",
      "OnCallGroup": "cn=lg-oncall"
    },
    "Slack": {
      "Enabled": true,
      "Channels": ["C0CRTBR8R"],
      "PostMessage": true
    }
  }
}
```

## Contributing
### Before you Begin
Before you start contributing to any project sponsored by F5, Inc. (F5) on GitHub, you will need to sign a Contributor License Agreement (CLA). This document can be provided to you once you submit a GitHub issue that you contemplate contributing code to, or after you issue a pull request.

If you are signing as an individual, we recommend that you talk to your employer (if applicable) before signing the CLA since some employment agreements may have restrictions on your contributions to other projects. Otherwise by submitting a CLA you represent that you are legally entitled to grant the licenses recited therein.

If your employer has rights to intellectual property that you create, such as your contributions, you represent that you have received permission to make contributions on behalf of that employer, that your employer has waived such rights for your contributions, or that your employer has executed a separate CLA with F5.

If you are signing on behalf of a company, you represent that you are legally entitled to grant the license recited therein. You represent further that each employee of the entity that submits contributions is authorized to submit such contributions on behalf of the entity pursuant to the CLA.
### Contribution Ideas
* Source and Sink additions/updates
  * Take a look at `deputize.go` to see how sources and sinks work
  * Take a look at `config.go` to see how sources and sinks are configured
* Abstract secret storage
  * A previous version of Deputize would read its secrets from Hashicorp Vault
### Testing Locally
You can perform local testing with by using the AWS Lambda Docker Images. Replace `arm64` with `amd64` if you're using an x86-64 flavored processor.

1. Build a local binary with `GOOS=linux go build`
2. Create a container: `docker build --platform linux/arm64 -t deputize:test .`
3. Run the container: `docker run --platform linux/arm64 -p 9000:8080 deputize:test`
4. Put the deputize configuration in `config.json`
4. In another window, invoke the function: `curl "http://localhost:9000/2015-03-31/functions/function/invocations" -d @config.json`

To get LDAP connectivity working in the container, set `Server` to `host.docker.internal`. You may find the `InsecureSkipVerify` flag helpful in this case, but it's not something you'd want to deploy into production.

To get AWS Secret Manager integration working in your local container, you could add the following ENV vars to `Dockerfile` - just make sure to remove when you're done.
```
ENV AWS_ACCESS_KEY_ID "..."
ENV AWS_SECRET_ACCESS_KEY "..."
ENV AWS_SESSION_TOKEN "..."
```