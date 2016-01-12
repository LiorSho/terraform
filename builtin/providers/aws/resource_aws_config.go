package aws

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/configservice"
)

func resourceAwsConfigService() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsConfigServiceCreate,
		Read:   resourceAwsConfigServiceRead,
		Update: resourceAwsConfigServiceUpdate,
		Delete: resourceAwsConfigServiceDelete,

		Schema: map[string]*schema.Schema{
			"role_arn": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"delivery_frequency": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					value := v.(string)
					valid := false
					validValues := []string{configservice.MaximumExecutionFrequencyOneHour,
						configservice.MaximumExecutionFrequencyThreeHours,
						configservice.MaximumExecutionFrequencySixHours,
						configservice.MaximumExecutionFrequencyTwelveHours,
						configservice.MaximumExecutionFrequencyTwentyFourHours,
					}
					for _, vVal := range validValues {
						if value == vVal {
							valid = true
						}
					}

					if valid == false {
						errors = append(errors, fmt.Errorf(
							"%q must be one of: [%s]", k, strings.Join(validValues, ", ")))
					}
					return
				},
			},
			"s3_bucket_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"s3_key_prefix": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"sns_topic_arn": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceAwsConfigServiceCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).configserviceconn

	// "By default, AWS Config automatically assigns the name "default" when creating
	// the configuration recorder. You cannot change the assigned name."
	// http://docs.aws.amazon.com/sdk-for-go/api/service/configservice.html#type-DeliveryChannel
	var configName = "default"
	d.Set("name", configName)

	var roleArn string
	if v, ok := d.GetOk("role_arn"); ok {
		roleArn = v.(string)
	} else {
		return fmt.Errorf("aws_config requires role_arn to be specified")
	}

	configOpts := configservice.PutConfigurationRecorderInput{
		ConfigurationRecorder: &configservice.ConfigurationRecorder{ //Required
			Name:    aws.String(configName),
			RoleARN: aws.String(roleArn),
		},
	}

	_, putRecorderErr := conn.PutConfigurationRecorder(&configOpts)

	if putRecorderErr != nil {
		return fmt.Errorf("[FAILURE] Failed to create ConfigurationRecorder: %s", putRecorderErr)
	}

	deliveryOpts := configservice.PutDeliveryChannelInput{
		DeliveryChannel: &configservice.DeliveryChannel{
			Name: aws.String(configName),
		},
	}

	if v, ok := d.GetOk("delivery_frequency"); ok {
		deliveryOpts.DeliveryChannel.ConfigSnapshotDeliveryProperties = &configservice.ConfigSnapshotDeliveryProperties{
			DeliveryFrequency: aws.String(v.(string)),
		}
	}

	if v, ok := d.GetOk("s3_bucket_name"); ok {
		deliveryOpts.DeliveryChannel.S3BucketName = aws.String(v.(string))
	}

	if v, ok := d.GetOk("s3_key_prefix"); ok {
		deliveryOpts.DeliveryChannel.S3KeyPrefix = aws.String(v.(string))
	}

	if v, ok := d.GetOk("sns_topic_arn"); ok {
		deliveryOpts.DeliveryChannel.SnsTopicARN = aws.String(v.(string))
	}

	deliveryErr := putDeliveryChannelWithRetry(conn, &deliveryOpts, 1, 5)

	if deliveryErr != nil {
		return fmt.Errorf("[FAILURE] Failed to create DeliveryChannel: %s", deliveryErr)
	}

	startRecordingOpts := configservice.StartConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String(configName),
	}

	_, startErr := conn.StartConfigurationRecorder(&startRecordingOpts)

	if startErr != nil {
		return fmt.Errorf("Error starting ConfigurationRecorder: %s", startErr)
	}

	d.SetId(configName)
	log.Printf("[INFO] Config ID: %s", d.Id())

	return resourceAwsConfigServiceRead(d, meta)
}

func resourceAwsConfigServiceRead(d *schema.ResourceData, meta interface{}) error {
	config, err := getAwsConfigRecorder(d, meta)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}

	d.Set("name", config.Name)
	d.Set("role_arn", config.RoleARN)

	channel, err := getAwsDeliveryChannel(d, meta)
	if err != nil {
		return err
	}
	if channel == nil {
		return nil
	}

	d.Set("s3_bucket_name", channel.S3BucketName)
	d.Set("s3_key_prefix", channel.S3KeyPrefix)
	d.Set("sns_topic_arn", channel.SnsTopicARN)
	d.Set("delivery_frequency", channel.ConfigSnapshotDeliveryProperties.DeliveryFrequency)

	return nil
}

func resourceAwsConfigServiceUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).configserviceconn

	config, err := getAwsConfigRecorder(d, meta)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}

	configInput := &configservice.PutConfigurationRecorderInput{
		ConfigurationRecorder: config}

	if d.HasChange("role_arn") {
		configInput.ConfigurationRecorder.RoleARN = aws.String(d.Get("role_arn").(string))
	}

	_, putRecorderErr := conn.PutConfigurationRecorder(configInput)

	if putRecorderErr != nil {
		return fmt.Errorf("[FAILURE] Failed to update ConfigurationRecorder: %s", putRecorderErr)
	}

	dchannel, err := getAwsDeliveryChannel(d, meta)
	if err != nil {
		return err
	}
	if dchannel == nil {
		return nil
	}

	deliveryInput := configservice.PutDeliveryChannelInput{
		DeliveryChannel: dchannel}

	if d.HasChange("s3_bucket_name") {
		deliveryInput.DeliveryChannel.S3BucketName = aws.String(d.Get("s3_bucket_name").(string))
	}

	if d.HasChange("s3_key_prefix") {
		deliveryInput.DeliveryChannel.S3KeyPrefix = aws.String(d.Get("s3_key_prefix").(string))
	}

	if d.HasChange("sns_topic_arn") {
		deliveryInput.DeliveryChannel.SnsTopicARN = aws.String(d.Get("sns_topic_arn").(string))
	}

	if d.HasChange("delivery_frequency") {
		deliveryInput.DeliveryChannel.ConfigSnapshotDeliveryProperties = &configservice.ConfigSnapshotDeliveryProperties{
			DeliveryFrequency: aws.String(d.Get("delivery_frequency").(string)),
		}
	}

	_, deliveryErr := conn.PutDeliveryChannel(&deliveryInput)

	if deliveryErr != nil {
		return fmt.Errorf("[FAILURE] Failed to update DeliveryChannel: %s", deliveryErr)
	}

	return resourceAwsConfigServiceRead(d, meta)
}

func resourceAwsConfigServiceDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).configserviceconn

	// Read the config recorder first. If it doesn't exist, we're done.
	c, err := getAwsConfigRecorder(d, meta)
	if err != nil {
		return err
	}
	if c == nil {
		return nil
	}

	deleteOpts := configservice.StopConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String(d.Id()),
	}

	_, stoperr := conn.StopConfigurationRecorder(&deleteOpts)

	if stoperr != nil {
		return fmt.Errorf("Error stopping configuration recording: %s", stoperr)
	}

	channel, err := getAwsDeliveryChannel(d, meta)
	if err != nil {
		return err
	}
	if channel == nil {
		return nil
	}

	deleteDeliveryOpts := configservice.DeleteDeliveryChannelInput{
		DeliveryChannelName: aws.String(d.Id()),
	}

	_, delerr := conn.DeleteDeliveryChannel(&deleteDeliveryOpts)

	if delerr != nil {
		return delerr
	}

	return nil

}

func getAwsConfigRecorder(
	d *schema.ResourceData,
	meta interface{}) (*configservice.ConfigurationRecorder, error) {

	conn := meta.(*AWSClient).configserviceconn

	describeOpts := configservice.DescribeConfigurationRecordersInput{
		ConfigurationRecorderNames: []*string{aws.String(d.Id())},
	}

	describeConfig, err := conn.DescribeConfigurationRecorders(&describeOpts)

	if err != nil {
		configerr, ok := err.(awserr.Error)
		if ok {
			return nil, fmt.Errorf("[FAILURE] Failed to retrieve information about Config recorder: %s", configerr.Code())
			//d.SetId("")
		}

		return nil, fmt.Errorf("Error retrieving AutoScaling groups: %s", err)
	}

	// Search for the Config
	for idx, cfg := range describeConfig.ConfigurationRecorders {
		if *cfg.Name == d.Id() {
			return describeConfig.ConfigurationRecorders[idx], nil
		}
	}

	// Config not found
	d.SetId("")
	return nil, nil
}

func getAwsDeliveryChannel(
	d *schema.ResourceData,
	meta interface{}) (*configservice.DeliveryChannel, error) {

	conn := meta.(*AWSClient).configserviceconn

	describeOpts := configservice.DescribeDeliveryChannelsInput{
		DeliveryChannelNames: []*string{aws.String(d.Id())},
	}

	describeChannels, err := conn.DescribeDeliveryChannels(&describeOpts)

	if err != nil {
		descerr, ok := err.(awserr.Error)
		if ok {
			return nil, fmt.Errorf("[FAILURE] Failed to retrieve information about Config recorder: %s", descerr.Code())
			//d.SetId("")
		}

		return nil, fmt.Errorf("Error retrieving AutoScaling groups: %s", err)
	}

	// Search for the Config
	for idx, chnl := range describeChannels.DeliveryChannels {
		if *chnl.Name == d.Id() {
			return describeChannels.DeliveryChannels[idx], nil
		}
	}

	// Config not found
	d.SetId("")
	return nil, nil
}

func putDeliveryChannelWithRetry(conn *configservice.ConfigService,
	params *configservice.PutDeliveryChannelInput,
	count int,
	max int,
) error {
	// PutDeliveryChannel fails if the s3 bucket and roles associated with s3 and ConfigService
	// were recently created (like right before resource_aws_config). We retry a sepecified
	// number of times to let the resource "get ready" and then return an error if
	// it's really not working.

	_, deliveryErr := conn.PutDeliveryChannel(params)

	if deliveryErr != nil {
		if deliveryErr.(awserr.Error).Code() == "InsufficientDeliveryPolicyException" {
			if count != max {
				log.Printf("[DEBUG] PutDeliveryChannel failed, retrying in 1 second")
				time.Sleep(1 * time.Second)
				count += 1
				return putDeliveryChannelWithRetry(conn, params, count, max)
			} else {
				return deliveryErr
			}
		} else {
			return deliveryErr
		}
	}

	return nil
}
