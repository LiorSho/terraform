package aws

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/configservice"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAccAWSConfigService_basic(t *testing.T) {
	var config configservice.ConfigService

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSConfigServiceDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSConfigServiceConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckConfigServiceEnabled("aws_config.foobar", &config),
				),
			},
		},
	})
}

func testAccCheckConfigServiceEnabled(n string, config *configservice.ConfigService) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := testAccProvider.Meta().(*AWSClient).configserviceconn
		params := configservice.DescribeConfigurationRecorderStatusInput{
			ConfigurationRecorderNames: []*string{
				aws.String(rs.Primary.ID),
			},
		}

		resp, err := conn.DescribeConfigurationRecorderStatus(&params)
		if err != nil {
			return err
		}

		fmt.Printf("%s", resp)
		for _, status := range resp.ConfigurationRecordersStatus {
			if !*status.Recording {
				return fmt.Errorf("AWS Config not recording")
			}
		}

		return nil
	}
}

func testAccCheckAWSConfigServiceDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).configserviceconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_config" {
			continue
		}

		params := configservice.DescribeConfigurationRecorderStatusInput{
			ConfigurationRecorderNames: []*string{
				aws.String(rs.Primary.ID),
			},
		}

		resp, err := conn.DescribeConfigurationRecorderStatus(&params)
		if err != nil {
			return err
		}

		fmt.Printf("%s", resp)
		for _, status := range resp.ConfigurationRecordersStatus {
			if *status.Recording {
				return fmt.Errorf("AWS Config is still recording")
			}
		}
	}
	return nil
}

var configServiceRandInt = rand.New(rand.NewSource(time.Now().UnixNano())).Int()

var testAccAWSConfigServiceConfig = fmt.Sprintf(`
resource "aws_s3_bucket" "foo" {
  bucket = "tf-test-configservice-%d"
  acl = "private"
  force_destroy = true
}

resource "aws_iam_role" "config_role" {
  name = "tf-test-iamrole-%d"
  assume_role_policy = <<ASSUMEPOLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "Service": "config.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
ASSUMEPOLICY
}

resource "aws_iam_role_policy" "config_policy" {
  name = "tf-test-aimrole_configpolicy-%d"
  role = "${aws_iam_role.config_role.id}"
  policy = <<ROLEPOLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cloudtrail:DescribeTrails",
        "ec2:Describe*",
        "config:PutEvaluations",
        "cloudtrail:GetTrailStatus",
        "s3:GetObject",
        "iam:GetAccountAuthorizationDetails",
        "iam:GetGroup",
        "iam:GetGroupPolicy",
        "iam:GetPolicy",
        "iam:GetPolicyVersion",
        "iam:GetRole",
        "iam:GetRolePolicy",
        "iam:GetUser",
        "iam:GetUserPolicy",
        "iam:ListAttachedGroupPolicies",
        "iam:ListAttachedRolePolicies",
        "iam:ListAttachedUserPolicies",
        "iam:ListEntitiesForPolicy",
        "iam:ListGroupPolicies",
        "iam:ListGroupsForUser",
        "iam:ListInstanceProfilesForRole",
        "iam:ListPolicyVersions",
        "iam:ListRolePolicies",
        "iam:ListUserPolicies"
      ],
      "Resource": "*"
    }
  ]
}
ROLEPOLICY
}

resource "aws_iam_role_policy" "s3_policy" {
  name = "tf-test-aimrole_s3policy-%d"
  role = "${aws_iam_role.config_role.id}"
  policy = <<ROLEPOLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:*",
      "Resource": "*"
    }
  ]
}
ROLEPOLICY
}

resource "aws_sns_topic" "blah" {
  name = "tf-test-snstopic-%d"
}

resource "aws_config" "foobar" {
  role_arn = "${aws_iam_role.config_role.arn}"
  delivery_frequency = "TwentyFour_Hours"
  s3_bucket_name = "${aws_s3_bucket.foo.id}"
  sns_topic_arn = "${aws_sns_topic.blah.arn}"
}
`, configServiceRandInt, configServiceRandInt, configServiceRandInt, configServiceRandInt, configServiceRandInt)
