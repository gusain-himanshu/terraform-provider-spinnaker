package spinnaker

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/himanhsugusain/terraform-provider-spinnaker/spinnaker/api"
)

func TestAccResourceSourceSpinnakerApplication_basic(t *testing.T) {
	resourceName := "spinnaker_application.test"
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckSpinnakerApplicatioDestroy(t, resourceName),
		Steps: []resource.TestStep{
			{
				Config: testAccSpinnakerApplication_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSpinnakerApplicationExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "email", "acceptance@test.com"),
					resource.TestCheckResourceAttr(resourceName, "instance_port", strconv.Itoa(defaultInstancePort)),
				),
			},
		},
	})
}

func TestAccResourceSourceSpinnakerApplication_instancePort(t *testing.T) {
	resourceName := "spinnaker_application.test"
	rName := acctest.RandomWithPrefix("tf-acc-test")
	rPort := acctest.RandIntRange(1, 1<<16)
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckSpinnakerApplicatioDestroy(t, resourceName),
		Steps: []resource.TestStep{
			{
				Config: testAccSpinnakerApplication_instancePort(rName, rPort),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSpinnakerApplicationExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "email", "acceptance@test.com"),
					resource.TestCheckResourceAttr(resourceName, "instance_port", strconv.Itoa(rPort)),
				),
			},
		},
	})
}

func TestAccResourceSourceSpinnakerApplication_cloudProviders(t *testing.T) {
	resourceName := "spinnaker_application.test"
	rName := acctest.RandomWithPrefix("tf-acc-test")
	cloudProvider := "kubernetes"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckSpinnakerApplicatioDestroy(t, resourceName),
		Steps: []resource.TestStep{
			{
				Config: testAccSpinnakerApplication_cloudProvider(rName, cloudProvider),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSpinnakerApplicationExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "email", "acceptance@test.com"),
					resource.TestCheckResourceAttr(resourceName, "instance_port", strconv.Itoa(defaultInstancePort)),
					resource.TestCheckResourceAttr(resourceName, "cloud_providers.0", cloudProvider),
				),
			},
		},
	})
}

func testAccCheckSpinnakerApplicatioDestroy(t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Application not found, application: %s", n)
		}

		appName := rs.Primary.ID
		if appName == "" {
			return fmt.Errorf("No Application ID is set")
		}

		client := testAccProvider.Meta().(gateConfig).client
		app := &applicationRead{}

		for retries := 1; retries <= 5; retries++ {
			if err := api.GetApplication(client, appName, app); err != nil {
				if strings.Contains(err.Error(), "not found") {
					return nil
				}
				return err
			}
			retryInterval := time.Duration(1<<retries) * time.Second
			t.Logf("[INFO] Retring CheckDestroy in %s, retry count: %v", retryInterval, retries)
			time.Sleep(retryInterval)
		}

		return fmt.Errorf("Spinnaker Application still exists, application: %s", appName)
	}
}

func testAccCheckSpinnakerApplicationExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Application not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Application ID is set")
		}
		client := testAccProvider.Meta().(gateConfig).client
		err := resource.Retry(1*time.Minute, func() *resource.RetryError {
			_, resp, err := client.ApplicationControllerApi.GetApplicationUsingGET(client.Context, rs.Primary.ID, nil)
			if resp != nil {
				if resp.StatusCode == http.StatusNotFound {
					return resource.RetryableError(fmt.Errorf("application does not exit"))
				} else if resp.StatusCode != http.StatusOK {
					return resource.NonRetryableError(fmt.Errorf("encountered an error getting application, status code: %d", resp.StatusCode))
				}
			}
			if err != nil {
				return resource.NonRetryableError(err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("Unable to find Application after retries: %s", err)
		}
		return nil
	}
}

func testAccSpinnakerApplication_basic(rName string) string {
	return fmt.Sprintf(`
resource "spinnaker_application" "test" {
	name  = %q
	email = "acceptance@test.com"
}
`, rName)
}

func testAccSpinnakerApplication_instancePort(rName string, instance_port int) string {
	return fmt.Sprintf(`
resource "spinnaker_application" "test" {
	name          = %q
	email         = "acceptance@test.com"
	instance_port = %d
}
`, rName, instance_port)
}

// Use single cloud provider for testing
func testAccSpinnakerApplication_cloudProvider(rName string, provider string) string {
	return fmt.Sprintf(`
resource "spinnaker_application" "test" {
	name          =  %q
	email         =  "acceptance@test.com"
	cloud_providers = [%q]
}
`, rName, provider)
}

func TestValidateApplicationName(t *testing.T) {
	validNames := []string{
		"ValidName",
		"validname",
		"invalid-name",
	}
	for _, v := range validNames {
		_, errors := validateSpinnakerApplicationName(v, "application")
		if len(errors) != 0 {
			t.Fatalf("%q should be a valid Application name: %q", v, errors)
		}
	}

	invalidNames := []string{
		"invalid:name",
		"invalid name",
		"invalid_name",
		"",
	}

	for _, v := range invalidNames {
		_, errors := validateSpinnakerApplicationName(v, "application")
		if len(errors) == 0 {
			t.Fatalf("%q should be a valid Application name", v)
		}
	}
}
