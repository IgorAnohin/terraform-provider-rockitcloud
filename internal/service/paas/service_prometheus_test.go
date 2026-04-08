package paas

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
)

func TestResourceServicePrometheusIntegration(t *testing.T) {
	t.Parallel()

	resource := ResourceService()

	prometheusSchema, ok := resource.Schema[services.Prometheus.ServiceType()]
	if !ok {
		t.Fatalf("expected %q schema to be registered", services.Prometheus.ServiceType())
	}

	if prometheusSchema == nil {
		t.Fatalf("expected %q schema to be non-nil", services.Prometheus.ServiceType())
	}

	resourceData := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
		services.Prometheus.ServiceType(): []interface{}{
			map[string]interface{}{
				"class": services.ServiceClassMonitoring,
			},
		},
	})

	manager := serviceManager(resourceData)
	if manager == nil {
		t.Fatal("expected prometheus service manager, got nil")
	}

	if got := manager.ServiceType(); got != services.Prometheus.ServiceType() {
		t.Fatalf("unexpected service type: got %q want %q", got, services.Prometheus.ServiceType())
	}
}
