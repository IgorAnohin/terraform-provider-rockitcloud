package paas

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
)

func TestResourceServiceELKIntegration(t *testing.T) {
	t.Parallel()

	resource := ResourceService()
	elkSchema, ok := resource.Schema[services.ServiceTypeELK]
	if !ok || elkSchema == nil {
		t.Fatalf("expected %q resource schema to be registered", services.ServiceTypeELK)
	}

	resourceData := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
		services.ServiceTypeELK: []interface{}{
			map[string]interface{}{
				"class":   services.ServiceClassLogging,
				"version": "8.17",
			},
		},
	})

	manager := serviceManager(resourceData)
	if manager == nil {
		t.Fatal("expected ELK service manager, got nil")
	}
	if got := manager.ServiceType(); got != services.ServiceTypeELK {
		t.Fatalf("unexpected service type: got %q want %q", got, services.ServiceTypeELK)
	}

	if !containsString(elkSchema.ExactlyOneOf, services.ServiceTypeELK) {
		t.Fatalf("ELK ExactlyOneOf does not contain itself: %#v", elkSchema.ExactlyOneOf)
	}
	if !containsString(elkSchema.ExactlyOneOf, services.ServiceTypeElasticSearch) {
		t.Fatalf("ELK must conflict with Elasticsearch through ExactlyOneOf: %#v", elkSchema.ExactlyOneOf)
	}
	if !reflect.DeepEqual(elkSchema.RequiredWith, []string{"data_volume"}) {
		t.Fatalf("unexpected ELK RequiredWith: %#v", elkSchema.RequiredWith)
	}
	if !containsString(elkSchema.ConflictsWith, "backup_settings") {
		t.Fatalf("ELK must conflict with backup_settings: %#v", elkSchema.ConflictsWith)
	}
	if containsString(elkSchema.ConflictsWith, "arbitrator_required") {
		t.Fatalf("ELK must allow arbitrator_required: %#v", elkSchema.ConflictsWith)
	}
}

func TestDataSourceServiceELKIntegration(t *testing.T) {
	t.Parallel()

	dataSource := DataSourceService()
	elkSchema, ok := dataSource.Schema[services.ServiceTypeELK]
	if !ok || elkSchema == nil {
		t.Fatalf("expected %q data source schema to be registered", services.ServiceTypeELK)
	}
	if !elkSchema.Computed {
		t.Fatal("ELK data source block must be computed")
	}

	nested := elkSchema.Elem.(*schema.Resource).Schema
	for _, name := range []string{
		"allow_anonymous",
		"anonymous_role",
		"class",
		"monitoring",
		"options",
		"password",
		"version",
	} {
		if _, ok := nested[name]; !ok {
			t.Errorf("expected %q in ELK data source schema", name)
		}
	}
}

func TestExistingPaaSManagersRemainRegisteredWithELK(t *testing.T) {
	t.Parallel()

	for _, serviceType := range []string{
		services.ServiceTypeElasticSearch,
		services.ServiceTypePrometheus,
		services.ServiceTypeELK,
	} {
		manager := services.Manager(serviceType)
		if manager == nil {
			t.Fatalf("expected manager for %q", serviceType)
		}
		if manager.ServiceType() != serviceType {
			t.Fatalf("unexpected manager for %q: %q", serviceType, manager.ServiceType())
		}
	}
}

func TestPreserveELKPasswordWhenAPIElidesIt(t *testing.T) {
	t.Parallel()

	resourceData := schema.TestResourceDataRaw(t, ResourceService().Schema, map[string]interface{}{
		services.ServiceTypeELK: []interface{}{
			map[string]interface{}{
				"class":    services.ServiceClassLogging,
				"password": "abcdefgh",
				"version":  "8.17",
			},
		},
	})

	for _, parametersMap := range []map[string]interface{}{
		{"version": "8.17"},
		{"password": "", "version": "8.17"},
	} {
		preserveELKPassword(resourceData, parametersMap)
		if err := resourceData.Set(services.ServiceTypeELK, []map[string]interface{}{parametersMap}); err != nil {
			t.Fatalf("setting refreshed ELK parameters: %s", err)
		}
		if got := resourceData.Get(services.ServiceTypeELK + ".0.password"); got != "abcdefgh" {
			t.Fatalf("password was not preserved after refresh: got %#v", got)
		}
	}
}

func TestPreserveELKPasswordUsesAPIValue(t *testing.T) {
	t.Parallel()

	resourceData := schema.TestResourceDataRaw(t, ResourceService().Schema, map[string]interface{}{
		services.ServiceTypeELK: []interface{}{
			map[string]interface{}{
				"class":    services.ServiceClassLogging,
				"password": "abcdefgh",
				"version":  "8.17",
			},
		},
	})
	parametersMap := map[string]interface{}{
		"password": "ijklmnop",
		"version":  "8.17",
	}

	preserveELKPassword(resourceData, parametersMap)
	if err := resourceData.Set(services.ServiceTypeELK, []map[string]interface{}{parametersMap}); err != nil {
		t.Fatalf("setting refreshed ELK parameters: %s", err)
	}
	if got := resourceData.Get(services.ServiceTypeELK + ".0.password"); got != "ijklmnop" {
		t.Fatalf("API password must win during refresh: got %#v", got)
	}
}

func TestResourceServiceELKEditableParametersDoNotRequireReplacement(t *testing.T) {
	t.Parallel()

	resource := ResourceService()
	initialConfig := testELKServiceConfig(map[string]interface{}{
		"version": "8.17",
		"options": map[string]interface{}{
			"node.attr.qa": "first",
		},
	})
	resourceData := schema.TestResourceDataRaw(t, resource.Schema, initialConfig)
	resourceData.SetId("fm-cluster-12345678")
	state := resourceData.State()

	for name, elkParameters := range map[string]map[string]interface{}{
		"options": {
			"version": "8.17",
			"options": map[string]interface{}{
				"node.attr.qa": "second",
			},
		},
		"monitoring": {
			"version": "8.17",
			"options": map[string]interface{}{
				"node.attr.qa": "first",
			},
			"monitoring": []interface{}{
				map[string]interface{}{
					"monitor_by": "fm-cluster-monitor",
				},
			},
		},
	} {
		name, elkParameters := name, elkParameters
		t.Run(name, func(t *testing.T) {
			diff, err := resource.Diff(
				context.Background(),
				state,
				terraform.NewResourceConfigRaw(testELKServiceConfig(elkParameters)),
				nil,
			)
			if err != nil {
				t.Fatalf("calculating ELK diff: %s", err)
			}
			if diff == nil || diff.Empty() {
				t.Fatalf("%s update unexpectedly produced an empty diff", name)
			}
			if diff.RequiresNew() {
				t.Fatalf("%s update unexpectedly requires ELK replacement: %#v", name, diff.Attributes)
			}
		})
	}
}

func testELKServiceConfig(elkParameters map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":          "tf-elk-test",
		"instance_type": "c5.large",
		"root_volume": []interface{}{
			map[string]interface{}{
				"type": "gp2",
				"size": 32,
			},
		},
		"data_volume": []interface{}{
			map[string]interface{}{
				"type": "gp2",
				"size": 32,
			},
		},
		"subnet_ids": []interface{}{"subnet-12345678"},
		services.ServiceTypeELK: []interface{}{
			elkParameters,
		},
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
