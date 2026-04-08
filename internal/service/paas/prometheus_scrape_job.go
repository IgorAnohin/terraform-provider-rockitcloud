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

func ResourcePrometheusScrapeJob() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePrometheusScrapeJobCreate,
		ReadContext:   resourcePrometheusScrapeJobRead,
		UpdateContext: resourcePrometheusScrapeJobUpdate,
		DeleteContext: resourcePrometheusScrapeJobDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourcePrometheusScrapeJobImport,
		},

		Schema: map[string]*schema.Schema{
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"job_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 256),
			},
			"targets": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringLenBetween(1, 2048),
				},
			},
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringLenBetween(1, 2048),
				},
			},
		},
	}
}

func resourcePrometheusScrapeJobCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &sdkpaas.CreatePrometheusScrapeJobInput{
		Name:       aws.String(d.Get("name").(string)),
		Parameters: expandPrometheusScrapeJobParameters(d),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Creating PaaS Prometheus Scrape Job: %v", input)
	output, err := conn.CreatePrometheusScrapeJob(input)
	if err != nil {
		return diag.Errorf("error creating PaaS Prometheus Scrape Job for service (%s): %s", serviceID, err)
	}

	d.SetId(aws.StringValue(output.PrometheusScrapeJob.Id))

	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutCreate)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after creating scrape job: %s", serviceID, err)
	}

	return resourcePrometheusScrapeJobRead(ctx, d, meta)
}

func resourcePrometheusScrapeJobRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	job, err := FindPrometheusScrapeJobByID(ctx, conn, serviceID, d.Id())
	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] PaaS Prometheus Scrape Job (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.Errorf("error reading PaaS Prometheus Scrape Job (%s): %s", d.Id(), err)
	}

	d.Set("job_id", aws.StringValue(job.Id))
	d.Set("name", aws.StringValue(job.Name))
	d.Set("targets", flex.FlattenStringSet(aws.StringSlice(getStringSliceParameter(job.Parameters, "targets"))))
	d.Set("labels", flattenStringMap(getStringMapParameter(job.Parameters, "labels")))

	return nil
}

func resourcePrometheusScrapeJobUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &sdkpaas.ModifyPrometheusScrapeJobInput{
		JobId:      aws.String(d.Id()),
		Parameters: expandPrometheusScrapeJobParameters(d),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Modifying PaaS Prometheus Scrape Job: %v", input)
	if _, err := conn.ModifyPrometheusScrapeJob(input); err != nil {
		return diag.Errorf("error modifying PaaS Prometheus Scrape Job (%s): %s", d.Id(), err)
	}

	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutUpdate)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after modifying scrape job (%s): %s", serviceID, d.Id(), err)
	}

	return resourcePrometheusScrapeJobRead(ctx, d, meta)
}

func resourcePrometheusScrapeJobDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	input := &sdkpaas.DeletePrometheusScrapeJobInput{
		JobId:     aws.String(d.Id()),
		ServiceId: aws.String(d.Get("service_id").(string)),
	}

	log.Printf("[DEBUG] Deleting PaaS Prometheus Scrape Job: %v", input)
	_, err := conn.DeletePrometheusScrapeJob(input)
	if isPrometheusNotFoundError(err) {
		return nil
	}
	if err != nil {
		return diag.Errorf("error deleting PaaS Prometheus Scrape Job (%s): %s", d.Id(), err)
	}

	serviceID := d.Get("service_id").(string)
	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after deleting scrape job (%s): %s", serviceID, d.Id(), err)
	}

	return nil
}

func resourcePrometheusScrapeJobImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	serviceID, jobID, err := parsePrometheusChildResourceImportID(d.Id(), "job")
	if err != nil {
		return nil, err
	}

	d.SetId(jobID)
	if err := d.Set("service_id", serviceID); err != nil {
		return nil, err
	}
	if err := d.Set("job_id", jobID); err != nil {
		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}

func expandPrometheusScrapeJobParameters(d *schema.ResourceData) map[string]interface{} {
	parameters := map[string]interface{}{}

	if v, ok := d.GetOk("targets"); ok {
		parameters["targets"] = expandStringSet(v)
	}
	if v, ok := d.GetOk("labels"); ok {
		parameters["labels"] = v.(map[string]interface{})
	}

	return parameters
}
