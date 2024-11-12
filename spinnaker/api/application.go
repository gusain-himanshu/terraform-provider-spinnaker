package api

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/mapstructure"
	gate "github.com/spinnaker/spin/cmd/gateclient"
	orca_tasks "github.com/spinnaker/spin/cmd/orca-tasks"
	gateclient "github.com/spinnaker/spin/gateapi"
)

var (
	// CloudProviders ...
	// See details in Spinnaker Orca
	// ref: https://github.com/spinnaker/orca/blob/master/orca-applications/src/main/groovy/com/netflix/spinnaker/orca/applications/utils/ApplicationNameValidator.groovy
	CloudProviders = map[string]applicationNameConstraint{
		"appengine":    {58, `^[a-z0-9]*$`},
		"aws":          {250, `^[a-zA-Z_0-9.]*$`},
		"dcos":         {127, `^[a-z0-9]*$`},
		"kubernetes":   {63, `^([a-zA-Z][a-zA-Z0-9-]*)$`},
		"gce":          {63, `^([a-zA-Z][a-zA-Z0-9]*)?$`},
		"openstack":    {250, `^[a-zA-Z_0-9.]*$`},
		"tencentcloud": {50, `^[a-zA-Z_0-9.\u4e00-\u9fa5-]*$`},
		"titus":        {250, `^[a-zA-Z_0-9.]*$`},
	}

	// SupportedAccesses is a list for Spinnaker application level
	// See details here
	// ref: https://spinnaker.io/setup/security/authorization/
	SupportedAccesses = []string{"READ", "WRITE", "EXECUTE"}
)

// applicationNameConstraint ...
type applicationNameConstraint struct {
	maxLength int
	regex     string
}

// CreateApplicationTask represents the Spinnaker createApplication Application API object
type CreateApplicationTask map[string]interface{}

// NewCreateApplicationTask returns a Spinnaker createApplication Application API object
// by passed resource data configured
func NewCreateApplicationTask(d *schema.ResourceData) (CreateApplicationTask, error) {
	app := map[string]interface{}{}
	app["name"] = GetApplicationName(d)
	app["email"] = d.Get("email").(string)
	app["instancePort"] = d.Get("instance_port").(int)

	if v, ok := d.GetOk("cloud_providers"); ok {
		input := v.([]interface{})
		cloudProviders := make([]string, len(input))
		for k, input := range v.([]interface{}) {
			cloudProvider := input.(string)
			if err := validateSpinnakerApplicationNameByCloudProvider(GetApplicationName(d), cloudProvider); err != nil {
				return nil, err
			}

			cloudProviders[k] = cloudProvider
		}

		app["cloudProviders"] = strings.Join(cloudProviders, ",")
	}

	if v, ok := d.GetOk("permission"); ok {
		permissions := map[string][]string{}

		inputs := v.([]interface{})
		for _, input := range inputs {
			input := input.(map[string]interface{})
			accesses := convToStringArray(input["accesses"].([]interface{}))
			if err := validateSpinnakerApplicationAccess(accesses); err != nil {
				return nil, err
			}
			for _, access := range accesses {
				if user := input["user"].(string); user != "" {
					if len(permissions[access]) == 0 {
						permissions[access] = []string{user}
						continue
					}

					if users := permissions[access]; len(users) > 0 {
						for _, v := range users {
							if user == v {
								return nil, fmt.Errorf("user %s permission's declare duplicated", user)
							}
						}
					}

					permissions[access] = append(permissions[access], user)
				}
			}
		}

		app["permissions"] = permissions
	}

	createAppTask := map[string]interface{}{
		"job":         []interface{}{map[string]interface{}{"type": "createApplication", "application": app}},
		"application": app["name"],
		"description": fmt.Sprintf("Create Application: %s", app["name"]),
	}

	return createAppTask, nil
}

// GetApplication gets an application from Spinnaker Gate
func GetApplication(client *gate.GatewayClient, appName string, dest interface{}) error {
	opts := &gateclient.ApplicationControllerApiGetApplicationUsingGETOpts{}
	app, resp, err := client.ApplicationControllerApi.GetApplicationUsingGET(client.Context, appName, opts)
	if resp != nil {
		if resp.StatusCode == http.StatusNotFound {
			return ErrCodeNoSuchEntityException
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("encountered an error getting application, status code: %d", resp.StatusCode)
		}
	}

	if err != nil {
		log.Println("app", app)
		log.Println("resp", resp)
		log.Println("err", err)
		return err
	}

	if err := mapstructure.Decode(app, dest); err != nil {
		return err
	}

	return nil
}

// CreateApplication creates passed application
func CreateApplication(client *gate.GatewayClient, createAppTask CreateApplicationTask) error {
	ref, _, err := client.TaskControllerApi.TaskUsingPOST1(client.Context, createAppTask)
	if err != nil {
		return err
	}
	return orca_tasks.WaitForSuccessfulTask(client, ref)
}

// DeleteApplication deletes an application by application name
func DeleteApplication(client *gate.GatewayClient, appName string) error {
	jobSpec := map[string]interface{}{
		"type": "deleteApplication",
		"application": map[string]interface{}{
			"name": appName,
		},
	}

	deleteAppTask := map[string]interface{}{
		"job":         []interface{}{jobSpec},
		"application": appName,
		"description": fmt.Sprintf("Delete Application: %s", appName),
	}

	ref, _, err := client.TaskControllerApi.TaskUsingPOST1(client.Context, deleteAppTask)
	if err != nil {
		return err
	}
	return orca_tasks.WaitForSuccessfulTask(client, ref)

}

func validateSpinnakerApplicationNameByCloudProvider(appName, provider string) error {
	if regex, ok := CloudProviders[provider]; ok {
		if !regexp.MustCompile(regex.regex).MatchString(appName) {
			return fmt.Errorf("application name %s for cloud provider %s doesn't match %s", appName, provider, regex.regex)
		}

		if c := len(appName); c > regex.maxLength {
			return fmt.Errorf("application name %s for cloud provider %s is more than its limit %d", appName, provider, c)
		}

		return nil
	}

	return fmt.Errorf("cloud provider %s is not supported", provider)
}

func validateSpinnakerApplicationAccess(accesses []string) error {
	for _, access := range accesses {
		var validAccess bool
		for _, v := range SupportedAccesses {
			if access == v {
				validAccess = true
			}
		}

		if !validAccess {
			return fmt.Errorf("access %s is not supported", access)
		}
	}

	return nil
}

func convToStringArray(in []interface{}) []string {
	out := make([]string, len(in))
	for i, v := range in {
		if str, ok := v.(string); ok {
			out[i] = str
		}
	}

	return out
}

func GetApplicationName(d *schema.ResourceData) string {
	name := d.Get("name").(string)
	if name == "" {
		if name = d.Get("application").(string); name == "" {
			name = d.Id()
		}
	}

	return name
}
