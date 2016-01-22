---
layout: "aws"
page_title: "AWS: aws_config"
sidebar_current: "docs-aws-resource-config"
description: |-
  Provides an AWS Config resource.
---

# aws\_config

Provides an AWS Config resource.

## Example Usage

```
resource "aws_config" "default" {
  role_arn = "${aws_iam_role.config_role.arn}"
  delivery_frequency = "TwentyFour_Hours"
  s3_bucket_name = "${aws_s3_bucket.foo.id}"
  s3_key_prefix = "config-logs"
  sns_topic_arn = "${aws_sns_topic.blah.arn}"
}
```

~> **Note:** AWS only supports one configuration recorder per account.
Additionally, this resource doesn't currently support specifying the recordingGroup parameter.
The default is to record all supported resource types.



## Argument Reference

For more detailed documentation about each argument, refer to
the [AWS official documentation](http://docs.aws.amazon.com/cli/latest/reference/configservice/index.html).

The following arguments are supported:

* `role_arn` - (Required) Amazon Resource Name (ARN) of the IAM role used to describe the AWS resources associated with the account.
* `delivery_frequency` - (Optional) The frequency with which a AWS Config recurringly delivers configuration snapshots. See the [AWS documentation](http://docs.aws.amazon.com/config/latest/APIReference/API_ConfigSnapshotDeliveryProperties.html) for valid values.
* `s3_bucket_name` - (Optional) The name of the Amazon S3 bucket used to store configuration history for the delivery channel.
* `s3_key_prefix` - (Optional) The prefix for the specified Amazon S3 bucket.
* `sns_topic_arn` - (Optional) The Amazon Resource Name (ARN) of the SNS topic that AWS Config delivers notifications to.
