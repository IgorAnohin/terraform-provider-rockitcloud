package paas

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	sdkpaas "github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourcePrometheusRoute() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePrometheusRouteCreate,
		ReadContext:   resourcePrometheusRouteRead,
		UpdateContext: resourcePrometheusRouteUpdate,
		DeleteContext: resourcePrometheusRouteDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourcePrometheusRouteImport,
		},

		Schema: map[string]*schema.Schema{
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"route_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 256),
			},
			"receiver": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 256),
			},
			"matchers": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringLenBetween(1, 2048),
				},
			},
			"continue": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"group_by": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringLenBetween(1, 256),
				},
			},
			"group_wait": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 128),
			},
			"group_interval": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 128),
			},
			"repeat_interval": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 128),
			},
		},
	}
}

func resourcePrometheusRouteCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &sdkpaas.CreatePrometheusRouteInput{
		Name:       aws.String(d.Get("name").(string)),
		Parameters: expandPrometheusRouteParameters(d),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Creating PaaS Prometheus Route: %v", input)
	output, err := conn.CreatePrometheusRoute(input)
	if err != nil {
		return diag.Errorf("error creating PaaS Prometheus Route for service (%s): %s", serviceID, err)
	}

	d.SetId(aws.StringValue(output.PrometheusRoute.Id))

	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutCreate)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after creating route: %s", serviceID, err)
	}

	return resourcePrometheusRouteRead(ctx, d, meta)
}

func resourcePrometheusRouteRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	route, err := FindPrometheusRouteByID(ctx, conn, serviceID, d.Id())
	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] PaaS Prometheus Route (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.Errorf("error reading PaaS Prometheus Route (%s): %s", d.Id(), err)
	}

	parameters := route.Parameters

	d.Set("route_id", aws.StringValue(route.Id))
	d.Set("name", aws.StringValue(route.Name))
	d.Set("receiver", getStringParameter(parameters, "receiver"))
	d.Set("matchers", flex.FlattenStringSet(aws.StringSlice(getStringSliceParameter(parameters, "matchers"))))
	d.Set("group_by", flex.FlattenStringSet(aws.StringSlice(getStringSliceParameter(parameters, "groupBy"))))
	d.Set("group_wait", getStringParameter(parameters, "groupWait"))
	d.Set("group_interval", getStringParameter(parameters, "groupInterval"))
	d.Set("repeat_interval", getStringParameter(parameters, "repeatInterval"))
	continueValue, continueSet := getBoolParameter(parameters, "continue")
	setOptionalBoolState(d, "continue", continueValue, continueSet)

	return nil
}

func resourcePrometheusRouteUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &sdkpaas.ModifyPrometheusRouteInput{
		Parameters: expandPrometheusRouteParameters(d),
		RouteId:    aws.String(d.Id()),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Modifying PaaS Prometheus Route: %v", input)
	if _, err := conn.ModifyPrometheusRoute(input); err != nil {
		return diag.Errorf("error modifying PaaS Prometheus Route (%s): %s", d.Id(), err)
	}

	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutUpdate)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after modifying route (%s): %s", serviceID, d.Id(), err)
	}

	return resourcePrometheusRouteRead(ctx, d, meta)
}

func resourcePrometheusRouteDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	input := &sdkpaas.DeletePrometheusRouteInput{
		RouteId:   aws.String(d.Id()),
		ServiceId: aws.String(d.Get("service_id").(string)),
	}

	log.Printf("[DEBUG] Deleting PaaS Prometheus Route: %v", input)
	_, err := conn.DeletePrometheusRoute(input)
	if isPrometheusNotFoundError(err) {
		return nil
	}
	if err != nil {
		return diag.Errorf("error deleting PaaS Prometheus Route (%s): %s", d.Id(), err)
	}

	serviceID := d.Get("service_id").(string)
	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after deleting route (%s): %s", serviceID, d.Id(), err)
	}

	return nil
}

func resourcePrometheusRouteImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	serviceID, routeID, err := parsePrometheusChildResourceImportID(d.Id(), "route")
	if err != nil {
		return nil, err
	}

	d.SetId(routeID)
	if err := d.Set("service_id", serviceID); err != nil {
		return nil, err
	}
	if err := d.Set("route_id", routeID); err != nil {
		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}

func expandPrometheusRouteParameters(d *schema.ResourceData) map[string]interface{} {
	parameters := map[string]interface{}{
		"receiver": d.Get("receiver").(string),
	}

	if v, ok := d.GetOk("matchers"); ok {
		parameters["matchers"] = expandStringSet(v)
	}
	parameters["continue"] = d.Get("continue").(bool)
	if v, ok := d.GetOk("group_by"); ok {
		parameters["groupBy"] = expandStringSet(v)
	}
	if v, ok := d.GetOk("group_wait"); ok {
		parameters["groupWait"] = v.(string)
	}
	if v, ok := d.GetOk("group_interval"); ok {
		parameters["groupInterval"] = v.(string)
	}
	if v, ok := d.GetOk("repeat_interval"); ok {
		parameters["repeatInterval"] = v.(string)
	}

	return parameters
}
