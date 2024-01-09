# pdrotator

PDRotator is a tool that can work with AWS Secret Manager to obtain a scoped OAuth token for PagerDuty. This allows you to limit the API access for Deputize. Now instead of requiring read access to all PD API elements, you can scope it to just `schedules.read`. Your security team will thank you.

## Prerequisites

You'll need to be, or have a PagerDuty account admin or owner around to complete this step.

In PagerDuty, register a new OAuth application - https://yourinstance.pagerduty.com/developer/applications. For authorization, specify **Scoped OAuth**. Next to schedules, select read. 

Save the Client ID and Client Secret. Also note the region of your PD instance (US or EU) for later.

## Installation
### Configuration
Create an AWS Secret Manager Secret that contains configuration for your PD instance(s). 

Name the secret `deputize/source/pagerduty` and load up the plaintext editor. The key for the JSON object should match your PD instance name. You can configure multiple PD instances, if you have them.

```
{
  "yourinstance": {
    "region": "us",
    "id": "PD_CLIENT_ID",
    "secret" :"PD_CLIENT_KEY_STARTS_WITH_PDEOC",
    "scopes": ["schedules.read"]
  }
}
```

### Create the Target Secret
Create a new secret: `deputize/source/pagerduty/yourinstance` with a value of `foo`. Don't enable rotation yet. Note the ARN of the secret, you'll need it for the IAM policy.

### IAM
Create a new IAM role for the lambda that will manage the PD OAuth token. PDRotator will need to log to CloudWatch, read it's config, read the target secret's values, and also write to the target secret. The IAM policy should look like the following:

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
            "Sid": "ReadConfigAndCurrentSecret",
            "Effect": "Allow",
            "Action": [
                "secretsmanager:DescribeSecret",
                "secretsmanager:GetSecretValue"
            ],
            "Resource": [
                "arn:aws:secretsmanager:REGION:AWS_ACCOUNT_NUM:secret:deputize/source/pagerduty-ID",
                "arn:aws:secretsmanager:REGION:AWS_ACCOUNT_NUM:secret:deputize/source/pagerduty/yourinstance-ID"
            ]
        },
        {
            "Sid": "UpdateSecret",
            "Effect": "Allow",
            "Action": [
                "secretsmanager:PutSecretValue",
                "secretsmanager:UpdateSecretVersionStage"
            ],
            "Resource": [
                "arn:aws:secretsmanager:REGION:AWS_ACCOUNT_NUM:secret:deputize/source/pagerduty/yourinstance-ID"
            ]
        }
    ]
}
```

### Upload the Lambda
* Build the function `GOOS=linux GOARCH=amd64 go build && zip pdrotator.zip pdrotator` and then upload it to AWS.
* Configure the handler to be `pdrotator`
* 128MB of memory is enough for this function
* Add a resource policy to allow `lambda:InvokeFunction` from `secretsmanager.amazonaws.com`

### Configure rotation
1. Go to your target secret - `deputize/source/pagerduty/yourinstance`.
2. Enable automatic rotation - every 23 hours should be sufficient. 
3. Point it at your pdrotator function.

Attempt rotation. You should get a secret returned that starts with `pdus+` or `pdeu+` -- success!

### Deputize Setup
1. Your Deputize IAM role will need to be updated to allow a read from the new target secret.
3. In your PagerDuty source configuration, make sure you have `WithOAuth` set to true, and `OAuthSecretPath` set to the target secret name.