package paas_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfpaas "github.com/hashicorp/terraform-provider-aws/internal/service/paas"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func TestAccPaaSELK_logstashPipelineBasic(t *testing.T) {
	resourceName := "aws_paas_logstash_pipeline.test"
	serviceResourceName := "aws_paas_service.test"
	serviceDataSourceName := "data.aws_paas_service.test"

	randomSuffix := sdkacctest.RandString(10)
	rName := fmt.Sprintf("elk-test-%s", randomSuffix)
	keyName := fmt.Sprintf("elk-test-key-%s", randomSuffix)
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	var pipelineBeforeUpdate, pipelineAfterUpdate, pipelineAfterRename *paas.LogstashPipeline

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acctest.PreCheck(t) },
		// PaaS's shared error check skips unsupported versions and instance types.
		// ELK validation must report every provider or cloud error as a test failure.
		ErrorCheck:        func(err error) error { return err },
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckELKDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLogstashPipelineConfig(
					rName,
					keyName,
					publicKey,
					"terraform-acceptance",
					testAccLogstashPipelineConfiguration("first"),
				),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(serviceResourceName),
					testAccCheckLogstashPipelineExists(resourceName, &pipelineBeforeUpdate),
					resource.TestCheckResourceAttr(serviceResourceName, "service_type", "elk"),
					resource.TestCheckResourceAttr(serviceResourceName, "service_class", "logging"),
					resource.TestCheckResourceAttr(serviceResourceName, "status", tfpaas.ServiceStatusReady),
					resource.TestCheckResourceAttr(serviceResourceName, "elk.#", "1"),
					resource.TestCheckResourceAttr(serviceResourceName, "elk.0.class", "logging"),
					resource.TestCheckResourceAttr(serviceResourceName, "elk.0.version", "8.17"),
					resource.TestCheckResourceAttr(serviceDataSourceName, "service_type", "elk"),
					resource.TestCheckResourceAttr(serviceDataSourceName, "service_class", "logging"),
					resource.TestCheckResourceAttr(serviceDataSourceName, "elk.0.class", "logging"),
					resource.TestCheckResourceAttr(serviceDataSourceName, "elk.0.version", "8.17"),
					resource.TestCheckResourceAttr(resourceName, "name", "terraform-acceptance"),
					resource.TestCheckResourceAttrSet(resourceName, "pipeline_id"),
					resource.TestCheckResourceAttrPair(resourceName, "pipeline_id", resourceName, "id"),
					resource.TestCheckResourceAttrPair(
						resourceName,
						"service_id",
						serviceResourceName,
						"id",
					),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccLogstashPipelineImportStateIDFunc(resourceName),
				ImportStateVerify: true,
			},
			{
				ResourceName:      serviceResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"arbitrator_required",
					"delete_interfaces_on_destroy",
				},
			},
			{
				Config: testAccLogstashPipelineConfig(
					rName,
					keyName,
					publicKey,
					"terraform-acceptance",
					testAccLogstashPipelineConfiguration("updated"),
				),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLogstashPipelineExists(resourceName, &pipelineAfterUpdate),
					testAccCheckLogstashPipelineNotRecreated(
						&pipelineBeforeUpdate,
						&pipelineAfterUpdate,
					),
					resource.TestCheckResourceAttr(
						resourceName,
						"configuration",
						testAccLogstashPipelineConfiguration("updated"),
					),
				),
			},
			{
				Config: testAccLogstashPipelineConfig(
					rName,
					keyName,
					publicKey,
					"terraform-acceptance-renamed",
					testAccLogstashPipelineConfiguration("updated"),
				),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLogstashPipelineExists(resourceName, &pipelineAfterRename),
					testAccCheckLogstashPipelineRecreated(
						&pipelineAfterUpdate,
						&pipelineAfterRename,
					),
					testAccCheckLogstashPipelineRemoved(
						resourceName,
						&pipelineAfterUpdate,
					),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"terraform-acceptance-renamed",
					),
				),
			},
			{
				Config: testAccLogstashPipelineConfig(
					rName,
					keyName,
					publicKey,
					"terraform-acceptance-renamed",
					testAccLogstashPipelineConfiguration("updated"),
				),
				Check: resource.ComposeTestCheckFunc(
					acctest.CheckResourceDisappears(
						acctest.Provider,
						tfpaas.ResourceLogstashPipeline(),
						resourceName,
					),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccLogstashPipelineConfig(
					rName,
					keyName,
					publicKey,
					"terraform-acceptance-renamed",
					testAccLogstashPipelineConfiguration("updated"),
				),
				Check: testAccCheckLogstashPipelineExists(resourceName, nil),
			},
		},
	})
}

func TestAccPaaSELK_parameters(t *testing.T) {
	resourceName := "aws_paas_service.test"
	dataSourceName := "data.aws_paas_service.test"

	monitorServiceID, securityGroupID, subnetID := testAccELKParameterEnvironment(t)
	randomSuffix := sdkacctest.RandString(10)
	rName := fmt.Sprintf("elk-test-%s", randomSuffix)
	keyName := fmt.Sprintf("elk-test-key-%s", randomSuffix)
	password := sdkacctest.RandString(16)
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	var serviceID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { acctest.PreCheck(t) },
		// ELK validation must report every provider or cloud error as a test failure.
		ErrorCheck:        func(err error) error { return err },
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccELKParametersConfig(
					rName,
					keyName,
					publicKey,
					password,
					monitorServiceID,
					securityGroupID,
					subnetID,
					"first",
					true,
				),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					testAccRememberServiceID(resourceName, &serviceID),
					testAccCheckELKInstanceCountAtLeast(resourceName, 1),
					resource.TestCheckResourceAttr(resourceName, "service_type", "elk"),
					resource.TestCheckResourceAttr(resourceName, "service_class", "logging"),
					resource.TestCheckResourceAttr(resourceName, "status", tfpaas.ServiceStatusReady),
					resource.TestCheckResourceAttr(resourceName, "high_availability", "false"),
					resource.TestCheckResourceAttr(resourceName, "arbitrator_required", "false"),
					resource.TestCheckResourceAttr(resourceName, "subnet_ids.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.version", "8.17"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.password", password),
					resource.TestCheckResourceAttr(resourceName, "elk.0.allow_anonymous", "true"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.anonymous_role.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.anonymous_role.0", "viewer"),
					resource.TestCheckResourceAttr(
						resourceName,
						"elk.0.options.node.attr.qa",
						"terraform",
					),
					resource.TestCheckResourceAttr(
						resourceName,
						"elk.0.monitoring.0.monitor_by",
						monitorServiceID,
					),
					resource.TestCheckResourceAttr(
						resourceName,
						"elk.0.monitoring.0.monitoring_labels.acceptance_phase",
						"first",
					),
					resource.TestCheckResourceAttrPair(dataSourceName, "id", resourceName, "id"),
					resource.TestCheckResourceAttr(dataSourceName, "service_type", "elk"),
					resource.TestCheckResourceAttr(dataSourceName, "service_class", "logging"),
					resource.TestCheckResourceAttr(dataSourceName, "status", tfpaas.ServiceStatusReady),
					resource.TestCheckResourceAttr(dataSourceName, "high_availability", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "instance_type", "m5.large"),
					testAccCheckELKInstanceCountAtLeast(dataSourceName, 1),
					resource.TestCheckResourceAttrSet(dataSourceName, "nodes.0.main.0.role"),
					resource.TestCheckResourceAttr(dataSourceName, "subnet_ids.#", "1"),
					resource.TestCheckTypeSetElemAttr(dataSourceName, "subnet_ids.*", subnetID),
					resource.TestCheckResourceAttr(dataSourceName, "security_group_ids.#", "1"),
					resource.TestCheckTypeSetElemAttr(
						dataSourceName,
						"security_group_ids.*",
						securityGroupID,
					),
					resource.TestCheckResourceAttr(dataSourceName, "ssh_key_name", keyName),
					resource.TestCheckResourceAttr(dataSourceName, "elk.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.class", "logging"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.version", "8.17"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.allow_anonymous", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.anonymous_role.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.anonymous_role.0", "viewer"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.monitoring.#", "1"),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitor_by",
						monitorServiceID,
					),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitoring_labels.%",
						"2",
					),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitoring_labels.acceptance_phase",
						"first",
					),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitoring_labels.managed_by",
						"terraform",
					),
				),
			},
			{
				Config: testAccELKParametersConfig(
					rName,
					keyName,
					publicKey,
					password,
					monitorServiceID,
					securityGroupID,
					subnetID,
					"second",
					true,
				),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					testAccCheckServiceIDUnchanged(resourceName, &serviceID),
					resource.TestCheckResourceAttr(
						resourceName,
						"elk.0.monitoring.0.monitoring_labels.acceptance_phase",
						"second",
					),
					resource.TestCheckResourceAttr(resourceName, "elk.0.password", password),
					resource.TestCheckResourceAttr(resourceName, "elk.0.allow_anonymous", "true"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.anonymous_role.0", "viewer"),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitor_by",
						monitorServiceID,
					),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitoring_labels.%",
						"2",
					),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitoring_labels.acceptance_phase",
						"second",
					),
					resource.TestCheckResourceAttr(
						dataSourceName,
						"elk.0.monitoring.0.monitoring_labels.managed_by",
						"terraform",
					),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.allow_anonymous", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.anonymous_role.0", "viewer"),
				),
			},
			{
				Config: testAccELKParametersConfig(
					rName,
					keyName,
					publicKey,
					password,
					monitorServiceID,
					securityGroupID,
					subnetID,
					"",
					false,
				),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					testAccCheckServiceIDUnchanged(resourceName, &serviceID),
					resource.TestCheckResourceAttr(resourceName, "elk.0.monitoring.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.password", password),
					resource.TestCheckResourceAttr(resourceName, "elk.0.allow_anonymous", "true"),
					resource.TestCheckResourceAttr(resourceName, "elk.0.anonymous_role.0", "viewer"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.monitoring.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.allow_anonymous", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "elk.0.anonymous_role.0", "viewer"),
				),
			},
		},
	})
}

func testAccELKParameterEnvironment(t *testing.T) (string, string, string) {
	t.Helper()

	monitorServiceID := os.Getenv("K2_ACC_ELK_MONITOR_SERVICE_ID")
	securityGroupID := os.Getenv("K2_ACC_ELK_SECURITY_GROUP_ID")
	subnetID := strings.TrimSpace(strings.Split(os.Getenv("K2_ACC_ELK_SUBNET_IDS"), ",")[0])
	if monitorServiceID == "" || securityGroupID == "" || subnetID == "" {
		t.Skip(
			"K2_ACC_ELK_MONITOR_SERVICE_ID, K2_ACC_ELK_SECURITY_GROUP_ID, " +
				"and K2_ACC_ELK_SUBNET_IDS are required",
		)
	}

	return monitorServiceID, securityGroupID, subnetID
}

func testAccRememberServiceID(resourceName string, serviceID *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS Service %s is not found in state", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("PaaS Service %s has an empty ID", resourceName)
		}

		*serviceID = rs.Primary.ID
		return nil
	}
}

func testAccCheckServiceIDUnchanged(
	resourceName string,
	serviceID *string,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if *serviceID == "" {
			return fmt.Errorf("cannot compare an empty PaaS Service ID")
		}
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS Service %s is not found in state", resourceName)
		}
		if rs.Primary.ID != *serviceID {
			return fmt.Errorf("PaaS Service was recreated: %s -> %s", *serviceID, rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckELKInstanceCountAtLeast(
	resourceName string,
	minimum int,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS Service %s is not found in state", resourceName)
		}
		count, err := strconv.Atoi(rs.Primary.Attributes["instances.#"])
		if err != nil {
			return fmt.Errorf("parse PaaS ELK instance count: %w", err)
		}
		if count < minimum {
			return fmt.Errorf("PaaS ELK has %d instances, want at least %d", count, minimum)
		}

		return nil
	}
}

func testAccCheckELKDestroy(s *terraform.State) error {
	if err := testAccCheckLogstashPipelineDestroy(s); err != nil {
		return err
	}

	return testAccCheckServiceDestroy(s)
}

func testAccCheckLogstashPipelineExists(
	resourceName string,
	pipeline **paas.LogstashPipeline,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS Logstash Pipeline %s is not found in state", resourceName)
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		output, err := tfpaas.FindLogstashPipelineByID(
			context.Background(),
			conn,
			rs.Primary.Attributes["service_id"],
			rs.Primary.ID,
		)
		if err != nil {
			return err
		}

		if pipeline != nil {
			*pipeline = output
		}

		return nil
	}
}

func testAccCheckLogstashPipelineDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_paas_logstash_pipeline" {
			continue
		}

		_, err := tfpaas.FindLogstashPipelineByID(
			context.Background(),
			conn,
			rs.Primary.Attributes["service_id"],
			rs.Primary.ID,
		)
		if tfresource.NotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("PaaS Logstash Pipeline (%s) still exists", rs.Primary.ID)
	}

	return nil
}

func testAccCheckLogstashPipelineNotRecreated(
	before **paas.LogstashPipeline,
	after **paas.LogstashPipeline,
) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if *before == nil || *after == nil {
			return fmt.Errorf("cannot compare nil PaaS Logstash Pipelines")
		}
		if aws.StringValue((*before).Id) != aws.StringValue((*after).Id) {
			return fmt.Errorf(
				"PaaS Logstash Pipeline was recreated: %s -> %s",
				aws.StringValue((*before).Id),
				aws.StringValue((*after).Id),
			)
		}

		return nil
	}
}

func testAccCheckLogstashPipelineRecreated(
	before **paas.LogstashPipeline,
	after **paas.LogstashPipeline,
) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if *before == nil || *after == nil {
			return fmt.Errorf("cannot compare nil PaaS Logstash Pipelines")
		}
		if aws.StringValue((*before).Id) == aws.StringValue((*after).Id) {
			return fmt.Errorf(
				"PaaS Logstash Pipeline was not recreated: %s",
				aws.StringValue((*after).Id),
			)
		}

		return nil
	}
}

func testAccCheckLogstashPipelineRemoved(
	resourceName string,
	pipeline **paas.LogstashPipeline,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if *pipeline == nil {
			return fmt.Errorf("cannot verify removal of a nil PaaS Logstash Pipeline")
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS Logstash Pipeline %s is not found in state", resourceName)
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		_, err := tfpaas.FindLogstashPipelineByID(
			context.Background(),
			conn,
			rs.Primary.Attributes["service_id"],
			aws.StringValue((*pipeline).Id),
		)
		if tfresource.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf(
			"replaced PaaS Logstash Pipeline (%s) still exists",
			aws.StringValue((*pipeline).Id),
		)
	}
}

func testAccLogstashPipelineImportStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("PaaS Logstash Pipeline %s is not found in state", resourceName)
		}

		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["service_id"], rs.Primary.ID), nil
	}
}

func testAccLogstashPipelineConfiguration(marker string) string {
	return fmt.Sprintf(
		`input { http { port => 4567 tags => ["terraform", "acceptance", %q] } }`,
		marker,
	)
}

func testAccLogstashPipelineConfig(
	serviceName string,
	keyName string,
	publicKey string,
	pipelineName string,
	pipelineConfiguration string,
) string {
	return fmt.Sprintf(`
resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = %[1]q
  }
}

resource "aws_internet_gateway" "test" {
  vpc_id = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_route" "test" {
  route_table_id         = aws_vpc.test.main_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.test.id
}

resource "aws_subnet" "test" {
  cidr_block = "10.0.1.0/24"
  vpc_id     = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_key_pair" "test" {
  key_name   = %[2]q
  public_key = %[3]q
}

resource "aws_paas_service" "test" {
  depends_on = [aws_route.test]

  name          = %[1]q
  instance_type = "m5.large"

  root_volume {
    type = "gp2"
    size = 32
  }

  data_volume {
    type = "gp2"
    size = 32
  }

  delete_interfaces_on_destroy = true
  security_group_ids           = [aws_vpc.test.default_security_group_id]
  subnet_ids                   = [aws_subnet.test.id]
  ssh_key_name                 = aws_key_pair.test.key_name

  elk {
    version = "8.17"
  }
}

resource "aws_paas_logstash_pipeline" "test" {
  service_id    = aws_paas_service.test.id
  name          = %[4]q
  configuration = %[5]q
}

data "aws_paas_service" "test" {
  id = aws_paas_service.test.id
}
`, serviceName, keyName, publicKey, pipelineName, pipelineConfiguration)
}

func testAccELKParametersConfig(
	serviceName string,
	keyName string,
	publicKey string,
	password string,
	monitorServiceID string,
	securityGroupID string,
	subnetID string,
	monitoringPhase string,
	monitoringEnabled bool,
) string {
	monitoringBlock := ""
	if monitoringEnabled {
		monitoringBlock = fmt.Sprintf(`
    monitoring {
      monitor_by = %q

      monitoring_labels = {
        acceptance_phase = %q
        managed_by       = "terraform"
      }
    }
`, monitorServiceID, monitoringPhase)
	}

	return fmt.Sprintf(`
resource "aws_key_pair" "test" {
  key_name   = %[2]q
  public_key = %[3]q
}

resource "aws_paas_service" "test" {
  name          = %[1]q
  instance_type = "m5.large"

  root_volume {
    type = "gp2"
    size = 32
  }

  data_volume {
    type = "gp2"
    size = 32
  }

  delete_interfaces_on_destroy   = true
  security_group_ids             = [%[5]q]
  subnet_ids                     = [%[6]q]
  ssh_key_name                   = aws_key_pair.test.key_name

  elk {
    version         = "8.17"
    password        = %[4]q
    allow_anonymous = true
    anonymous_role  = ["viewer"]

    options = {
      "node.attr.qa" = "terraform"
    }
%[7]s
  }
}

data "aws_paas_service" "test" {
  id = aws_paas_service.test.id
}
`,
		serviceName,
		keyName,
		publicKey,
		password,
		securityGroupID,
		subnetID,
		monitoringBlock,
	)
}
