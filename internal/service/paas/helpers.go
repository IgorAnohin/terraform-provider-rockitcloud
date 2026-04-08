package paas

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func parsePrometheusChildResourceImportID(id, resourceType string) (string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%q), expected service_id/%s_id", id, resourceType)
	}

	return parts[0], parts[1], nil
}

func expandStringSet(v interface{}) []string {
	if v == nil {
		return nil
	}

	values := make([]string, 0)
	for _, item := range v.(*schema.Set).List() {
		values = append(values, item.(string))
	}

	sort.Strings(values)

	return values
}

func flattenStringMap(v interface{}) map[string]interface{} {
	switch values := v.(type) {
	case map[string]interface{}:
		items := make(map[string]interface{}, len(values))
		for key, value := range values {
			if value == nil {
				continue
			}
			items[key] = fmt.Sprintf("%v", value)
		}
		return items
	case map[string]*string:
		items := make(map[string]interface{}, len(values))
		for key, value := range values {
			if value == nil {
				continue
			}
			items[key] = aws.StringValue(value)
		}
		return items
	default:
		return nil
	}
}

func getStringSliceParameter(parameters map[string]interface{}, key string) []string {
	raw, ok := parameters[key]
	if !ok || raw == nil {
		return nil
	}

	switch values := raw.(type) {
	case []string:
		out := append([]string(nil), values...)
		sort.Strings(out)
		return out
	case []*string:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if value == nil {
				continue
			}
			out = append(out, aws.StringValue(value))
		}
		sort.Strings(out)
		return out
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if value == nil {
				continue
			}
			out = append(out, fmt.Sprintf("%v", value))
		}
		sort.Strings(out)
		return out
	default:
		return nil
	}
}

func getStringMapParameter(parameters map[string]interface{}, key string) map[string]interface{} {
	raw, ok := parameters[key]
	if !ok || raw == nil {
		return nil
	}

	return flattenStringMap(raw)
}

func getStringParameter(parameters map[string]interface{}, key string) string {
	raw, ok := parameters[key]
	if !ok || raw == nil {
		return ""
	}

	switch value := raw.(type) {
	case string:
		return value
	case *string:
		return aws.StringValue(value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func getBoolParameter(parameters map[string]interface{}, key string) (bool, bool) {
	raw, ok := parameters[key]
	if !ok || raw == nil {
		return false, false
	}

	switch value := raw.(type) {
	case bool:
		return value, true
	case *bool:
		return aws.BoolValue(value), true
	default:
		return false, false
	}
}

func getIntParameter(parameters map[string]interface{}, key string) (int, bool) {
	raw, ok := parameters[key]
	if !ok || raw == nil {
		return 0, false
	}

	switch value := raw.(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case *int64:
		return int(aws.Int64Value(value)), true
	default:
		return 0, false
	}
}

func setOptionalBoolState(d *schema.ResourceData, key string, value bool, ok bool) {
	if ok {
		d.Set(key, value)
		return
	}

	d.Set(key, nil)
}

func setOptionalIntState(d *schema.ResourceData, key string, value int, ok bool) {
	if ok {
		d.Set(key, value)
		return
	}

	d.Set(key, nil)
}
