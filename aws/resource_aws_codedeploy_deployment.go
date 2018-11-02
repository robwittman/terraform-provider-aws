package aws

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/aws"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"log"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/hashicorp/terraform/helper/resource"
	"time"
)

func resourceAwsCodeDeployDeployment() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsCodeDeployDeploymentCreate,
		Read:   resourceAwsCodeDeployDeploymentRead,
		Delete: resourceAwsCodeDeployDeploymentDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"application_name": {
				Type: schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringLenBetween(1, 100),
			},
			"auto_rollback_configuration": {
				Type: schema.TypeMap,
				Required: false,
				ForceNew: false,
			},
			"deployment_config_name": {
				Type: schema.TypeString,
				Required: false,
				ForceNew: true,
				ValidateFunc: validation.StringLenBetween(1, 100),
			},
			"deployment_group_name": {
				Type: schema.TypeString,
				Required: false,
				ForceNew: true,
				ValidateFunc: validation.StringLenBetween(1, 100),
			},
			"description": {
				Type: schema.TypeString,
				Required: false,
				ForceNew: false,
			},
			"file_exists_behavior": {
				Type: schema.TypeString,
				Required: false,
				ForceNew: false,
				Default: "DISALLOW",
				ValidateFunc: validation.StringInSlice([]string{
					codedeploy.FileExistsBehaviorDisallow,
					codedeploy.FileExistsBehaviorOverwrite,
					codedeploy.FileExistsBehaviorRetain,
				}, false),
			},
			"ignore_application_stop_failures": {
				Type: schema.TypeBool,
				Required: false,
				ForceNew: false,
				Default: false,
			},
			"revision": {
				Type: schema.TypeList,
				Required: false,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"revision_type":   {
							Type:     schema.TypeString,
							Required: false,
							ForceNew: true,
							ValidateFunc: validation.StringInSlice([]string{
								codedeploy.RevisionLocationTypeGitHub,
								codedeploy.RevisionLocationTypeS3,
								codedeploy.RevisionLocationTypeString,
							}, false),
						},
						"github_location": {
							Type:     schema.TypeMap,
							Required: false,
							ForceNew: true,
							ConflictsWith: []string{"s3_location","revision_string"},
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"commit_id": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"repository": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
								},
							},
						},
						"s3_location":     {
							Type:     schema.TypeMap,
							Required: false,
							ForceNew: true,
							ConflictsWith: []string{"github_location","revision_string"},
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"bucket": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"bundle_type": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"key": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"e_tag": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"version": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
								},
							},
						},
						"string": {
							Type:     schema.TypeMap,
							Required: false,
							ForceNew: true,
							ConflictsWith: []string{"github_location","s3_location"},
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"content": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"sha256": {
										Type: schema.TypeString,
										Required: true,
										ForceNew: true,
									},
								},
							},
						},
					},
				},
			},

			"target_instances": {
				Type: schema.TypeMap,
				Required: false,
				ForceNew: false,
			},
			"update_outdated_instances_only": {
				Type: schema.TypeBool,
				Required: false,
				ForceNew: true,
				Default: false,
			},
		},
	}
}

func resourceAwsCodeDeployDeploymentCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).codedeployconn


	deploymentOpts, err := BuildDeploymentOpts(d, meta)
	if err != nil {
		return err
	}
	input := &codedeploy.CreateDeploymentInput{
		ApplicationName: deploymentOpts.ApplicationName,
		AutoRollbackConfiguration: deploymentOpts.AutoRollbackConfiguration,
		DeploymentConfigName: deploymentOpts.DeploymentConfigName,
		DeploymentGroupName: deploymentOpts.DeploymentGroupName,
		Description: deploymentOpts.Description,
		FileExistsBehavior: deploymentOpts.FileExistsBehavior,
		IgnoreApplicationStopFailures: deploymentOpts.IgnoreApplicationStopFailures,
		Revision: deploymentOpts.Revision,
		TargetInstances: deploymentOpts.TargetInstances,
		UpdateOutdatedInstancesOnly: deploymentOpts.UpdateOutdatedInstancesOnly,
	}
	deployment, err := conn.CreateDeployment(input)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{codedeploy.DeploymentStatusQueued},
		Target:     []string{codedeploy.DeploymentStatusSucceeded},
		Refresh:    DeploymentStateRefreshFunc(conn, *deployment.DeploymentId, []string{
			codedeploy.DeploymentStatusFailed,
			codedeploy.DeploymentStatusStopped}),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	deploymentRaw, err := stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf(
			"Error waiting for deployment (%s) to complete: %s",
			*deployment.DeploymentId, err)
	}
	deployment = deploymentRaw.(*codedeploy.CreateDeploymentOutput)

	return nil
}

func resourceAwsCodeDeployDeploymentDelete(d *schema.ResourceData, meta interface{}) error {
  // We don't actually delete anything :shrug:
  return nil
}

func resourceAwsCodeDeployDeploymentRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).codedeployconn

	input := &codedeploy.GetDeploymentInput{
		DeploymentId: aws.String(d.Id()),
	}

	resp, err := conn.GetDeployment(input)
	if (err) != nil {
		return err
	}

	if resp.DeploymentInfo == nil {
		return fmt.Errorf("Cannot find Deployment %q", d.Id())
	}

	d.Set("application_name", resp.DeploymentInfo.ApplicationName)
	d.Set("auto_rollback_configuration", resp.DeploymentInfo.AutoRollbackConfiguration)
	d.Set("deployment_config_name", resp.DeploymentInfo.DeploymentConfigName)
	d.Set("deployment_group_name", resp.DeploymentInfo.DeploymentGroupName)
	d.Set("description", resp.DeploymentInfo.Description)
	d.Set("file_exists_behavior", resp.DeploymentInfo.FileExistsBehavior)
	d.Set("ignore_application_stop_failures", resp.DeploymentInfo.IgnoreApplicationStopFailures)
	d.Set("revision", resp.DeploymentInfo.Revision)
	d.Set("target_instances", resp.DeploymentInfo.TargetInstances)
	d.Set("update_outdated_instances_only", resp.DeploymentInfo.UpdateOutdatedInstancesOnly)

	return nil
}

func DeploymentStateRefreshFunc(conn *codedeploy.CodeDeploy, deploymentId string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := conn.GetDeployment(&codedeploy.GetDeploymentInput{
			DeploymentId: aws.String(deploymentId),
		})

		if (err != nil) {
			return nil, "error", err
		}

		if (resp == nil) {
			return nil, "", nil
		}

		d := resp.DeploymentInfo


		state := aws.StringValue(d.Status)
		log.Printf("[DEBUG] current status of %q: %q", deploymentId, state)
		return d, state, nil
	}
}

type deploymentOpts struct {
	ApplicationName *string
	AutoRollbackConfiguration *codedeploy.AutoRollbackConfiguration
	DeploymentConfigName *string
	DeploymentGroupName *string
	Description *string
	FileExistsBehavior *string
	IgnoreApplicationStopFailures *bool
	Revision *codedeploy.RevisionLocation
	TargetInstances *codedeploy.TargetInstances
	UpdateOutdatedInstancesOnly *bool
}

func BuildDeploymentOpts(d *schema.ResourceData, meta interface{}) (*deploymentOpts, error) {
	opts := &deploymentOpts{
		ApplicationName: aws.String(d.Get("application_name").(string)),
		DeploymentGroupName: aws.String(d.Get("deployment_group_name").(string)),
		DeploymentConfigName: aws.String(d.Get("deployment_config_name").(string)),
		IgnoreApplicationStopFailures: aws.Bool(d.Get("ignore_application_stop_failures").(bool))
		UpdateOutdatedInstancesOnly: aws.Bool(d.Get("update_outdated_instances_only").(bool))
	}

	// Add AutoRollbackConfiguration

	if rollbackConfiguration, ok := d.GetOk("auto_rollback_configuration"); ok {
		configs := v.([]interface{})
		config, ok := configs[0].(map[string]interface{})
		if !ok {
			return errors.New("Rollback config should not be empty")
		}


		opts.AutoRollbackConfiguration = &codedeploy.AutoRollbackConfiguration{
			Enabled: aws.Bool(rollbackConfiguration["enabled"]),
			Events: rollbackConfiguration["events"]
		}
	}

	if rev, ok := d.GetOk("revision"); ok {

	}
	revisionType, ok := d.GetOk("revision_type")
	if ok {
		if revisionType == codedeploy.RevisionLocationTypeString {
			opts.Revision = &codedeploy.RevisionLocation{
				RevisionType: aws.String("String"),
			}
			opts.Revision.String_ = &codedeploy.RawString{
				Content: aws.String(""),
				Sha256:  aws.String(""),
			}
		} else if revisionType == codedeploy.RevisionLocationTypeS3 {
			opts.Revision = &codedeploy.RevisionLocation{
				RevisionType: aws.String("S3"),
			}
			opts.Revision.S3Location = &codedeploy.S3Location{
				Bucket:     aws.String(""),
				BundleType: aws.String(""),
				Version:    aws.String(""),
				ETag:       aws.String(""),
				Key:        aws.String(""),
			}
		} else if revisionType == codedeploy.RevisionLocationTypeGitHub {
			opts.Revision = &codedeploy.RevisionLocation{
				RevisionType: aws.String("GitHub"),
			}
			opts.Revision.GitHubLocation = &codedeploy.GitHubLocation{
				CommitId:   aws.String(""),
				Repository: aws.String("v"),
			}
		}
	}

	// Add TargetInstances
	if targetInstances, ok := d.GetOk("target_instances"); ok {
		opts.TargetInstances = &codedeploy.TargetInstances{}
	}

	return opts, nil
}