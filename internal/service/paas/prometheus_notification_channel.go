package paas

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	sdkpaas "github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

const (
	prometheusNotificationChannelTypeEmail    = "email"
	prometheusNotificationChannelTypeTelegram = "telegram"
	prometheusNotificationChannelTypeWebhook  = "webhook"
)

var (
	prometheusNotificationChannelTypes = []string{
		prometheusNotificationChannelTypeEmail,
		prometheusNotificationChannelTypeTelegram,
		prometheusNotificationChannelTypeWebhook,
	}

	prometheusTelegramNotificationChannelFields = []string{
		"bot_token",
		"chat_id",
	}
	prometheusWebhookNotificationChannelFields = []string{
		"url",
		"max_alerts",
	}
	prometheusEmailNotificationChannelFields = []string{
		"to",
		"from",
		"smarthost",
		"hello",
		"require_tls",
		"auth_username",
		"auth_password",
	}
)

func ResourcePrometheusNotificationChannel() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePrometheusNotificationChannelCreate,
		ReadContext:   resourcePrometheusNotificationChannelRead,
		UpdateContext: resourcePrometheusNotificationChannelUpdate,
		DeleteContext: resourcePrometheusNotificationChannelDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourcePrometheusNotificationChannelImport,
		},

		Schema: map[string]*schema.Schema{
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"channel_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 256),
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice(prometheusNotificationChannelTypes, false),
			},
			"is_default": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"send_resolved": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"bot_token": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				RequiredWith: []string{
					"chat_id",
				},
				ConflictsWith: append(
					append([]string{}, prometheusWebhookNotificationChannelFields...),
					prometheusEmailNotificationChannelFields...,
				),
			},
			"chat_id": {
				Type:     schema.TypeInt,
				Optional: true,
				RequiredWith: []string{
					"bot_token",
				},
				ConflictsWith: append(
					append([]string{}, prometheusWebhookNotificationChannelFields...),
					prometheusEmailNotificationChannelFields...,
				),
			},
			"url": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 2048),
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusEmailNotificationChannelFields...,
				),
			},
			"max_alerts": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(0),
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusEmailNotificationChannelFields...,
				),
			},
			// TODO: The current Prometheus parameter docs expose email.to as a single string value, not a list.
			"to": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 2048),
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
			"from": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 2048),
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
			"smarthost": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 2048),
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
			"hello": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 2048),
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
			"require_tls": {
				Type:     schema.TypeBool,
				Optional: true,
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
			"auth_username": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 2048),
				RequiredWith: []string{
					"auth_password",
				},
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
			"auth_password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				RequiredWith: []string{
					"auth_username",
				},
				ConflictsWith: append(
					append([]string{}, prometheusTelegramNotificationChannelFields...),
					prometheusWebhookNotificationChannelFields...,
				),
			},
		},
	}
}

func resourcePrometheusNotificationChannelCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := validatePrometheusNotificationChannelResourceData(d); err != nil {
		return diag.FromErr(err)
	}

	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &sdkpaas.CreateNotificationChannelInput{
		Name:       aws.String(d.Get("name").(string)),
		Parameters: expandPrometheusNotificationChannelParameters(d),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Creating PaaS Prometheus Notification Channel: %v", input)
	output, err := conn.CreateNotificationChannel(input)
	if err != nil {
		return diag.Errorf("error creating PaaS Prometheus Notification Channel for service (%s): %s", serviceID, err)
	}

	d.SetId(aws.StringValue(output.NotificationChannel.Id))

	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutCreate)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after creating notification channel: %s", serviceID, err)
	}

	return resourcePrometheusNotificationChannelRead(ctx, d, meta)
}

func resourcePrometheusNotificationChannelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	channel, err := FindPrometheusNotificationChannelByID(ctx, conn, serviceID, d.Id())
	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] PaaS Prometheus Notification Channel (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.Errorf("error reading PaaS Prometheus Notification Channel (%s): %s", d.Id(), err)
	}

	parameters := channel.Parameters

	d.Set("channel_id", aws.StringValue(channel.Id))
	d.Set("name", aws.StringValue(channel.Name))
	d.Set("type", getStringParameter(parameters, "type"))
	isDefault, isDefaultSet := getBoolParameter(parameters, "isDefault")
	setOptionalBoolState(d, "is_default", isDefault, isDefaultSet)
	sendResolved, sendResolvedSet := getBoolParameter(parameters, "sendResolved")
	setOptionalBoolState(d, "send_resolved", sendResolved, sendResolvedSet)
	chatID, chatIDSet := getIntParameter(parameters, "chatId")
	setOptionalIntState(d, "chat_id", chatID, chatIDSet)
	maxAlerts, maxAlertsSet := getIntParameter(parameters, "maxAlerts")
	setOptionalIntState(d, "max_alerts", maxAlerts, maxAlertsSet)
	d.Set("url", getStringParameter(parameters, "url"))
	d.Set("to", getStringParameter(parameters, "to"))
	d.Set("from", getStringParameter(parameters, "from"))
	d.Set("smarthost", getStringParameter(parameters, "smarthost"))
	d.Set("hello", getStringParameter(parameters, "hello"))
	d.Set("auth_username", getStringParameter(parameters, "authUsername"))
	requireTLS, requireTLSSet := getBoolParameter(parameters, "requireTls")
	setOptionalBoolState(d, "require_tls", requireTLS, requireTLSSet)

	// The Prometheus API may omit masked secrets on read. Only write them back to
	// state when the API actually returns a value so we do not invent placeholders
	// or create perpetual drift for config-managed sensitive fields.
	setPrometheusNotificationChannelSecretIfReturned(d, "bot_token", getStringParameter(parameters, "botToken"))
	setPrometheusNotificationChannelSecretIfReturned(d, "auth_password", getStringParameter(parameters, "authPassword"))

	return nil
}

func resourcePrometheusNotificationChannelUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := validatePrometheusNotificationChannelResourceData(d); err != nil {
		return diag.FromErr(err)
	}

	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &sdkpaas.ModifyNotificationChannelInput{
		ChannelId:  aws.String(d.Id()),
		Parameters: expandPrometheusNotificationChannelParameters(d),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Modifying PaaS Prometheus Notification Channel: %v", input)
	if _, err := conn.ModifyNotificationChannel(input); err != nil {
		return diag.Errorf("error modifying PaaS Prometheus Notification Channel (%s): %s", d.Id(), err)
	}

	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutUpdate)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after modifying notification channel (%s): %s", serviceID, d.Id(), err)
	}

	return resourcePrometheusNotificationChannelRead(ctx, d, meta)
}

func resourcePrometheusNotificationChannelDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	input := &sdkpaas.DeleteNotificationChannelInput{
		ChannelId: aws.String(d.Id()),
		ServiceId: aws.String(d.Get("service_id").(string)),
	}

	log.Printf("[DEBUG] Deleting PaaS Prometheus Notification Channel: %v", input)
	_, err := conn.DeleteNotificationChannel(input)
	if isPrometheusNotFoundError(err) {
		return nil
	}
	if err != nil {
		return diag.Errorf("error deleting PaaS Prometheus Notification Channel (%s): %s", d.Id(), err)
	}

	serviceID := d.Get("service_id").(string)
	if _, err := waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.Errorf("error waiting for PaaS Prometheus service (%s) to update after deleting notification channel (%s): %s", serviceID, d.Id(), err)
	}

	return nil
}

func resourcePrometheusNotificationChannelImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	serviceID, channelID, err := parsePrometheusChildResourceImportID(d.Id(), "channel")
	if err != nil {
		return nil, err
	}

	d.SetId(channelID)
	if err := d.Set("service_id", serviceID); err != nil {
		return nil, err
	}
	if err := d.Set("channel_id", channelID); err != nil {
		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}

func expandPrometheusNotificationChannelParameters(d *schema.ResourceData) map[string]interface{} {
	parameters := map[string]interface{}{
		"type":         d.Get("type").(string),
		"isDefault":    d.Get("is_default").(bool),
		"sendResolved": d.Get("send_resolved").(bool),
	}

	if v, ok := d.GetOk("bot_token"); ok {
		parameters["botToken"] = v.(string)
	}
	if v, ok := d.GetOk("chat_id"); ok {
		parameters["chatId"] = int64(v.(int))
	}
	if v, ok := d.GetOk("url"); ok {
		parameters["url"] = v.(string)
	}
	if v, ok := d.GetOk("max_alerts"); ok {
		parameters["maxAlerts"] = int64(v.(int))
	}
	if v, ok := d.GetOk("to"); ok {
		parameters["to"] = v.(string)
	}
	if v, ok := d.GetOk("from"); ok {
		parameters["from"] = v.(string)
	}
	if v, ok := d.GetOk("smarthost"); ok {
		parameters["smarthost"] = v.(string)
	}
	if v, ok := d.GetOk("hello"); ok {
		parameters["hello"] = v.(string)
	}
	if d.Get("type").(string) == prometheusNotificationChannelTypeEmail {
		parameters["requireTls"] = d.Get("require_tls").(bool)
	}
	if v, ok := d.GetOk("auth_username"); ok {
		parameters["authUsername"] = v.(string)
	}
	if v, ok := d.GetOk("auth_password"); ok {
		parameters["authPassword"] = v.(string)
	}

	return parameters
}

func validatePrometheusNotificationChannelResourceData(d *schema.ResourceData) error {
	channelType := d.Get("type").(string)

	switch channelType {
	case prometheusNotificationChannelTypeTelegram:
		if err := requirePrometheusNotificationChannelFields(d, channelType, prometheusTelegramNotificationChannelFields...); err != nil {
			return err
		}
	case prometheusNotificationChannelTypeWebhook:
		if err := requirePrometheusNotificationChannelFields(d, channelType, "url"); err != nil {
			return err
		}
	case prometheusNotificationChannelTypeEmail:
		if err := requirePrometheusNotificationChannelFields(d, channelType, "to", "from", "smarthost"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported Prometheus notification channel type %q", channelType)
	}

	return nil
}

func requirePrometheusNotificationChannelFields(d *schema.ResourceData, channelType string, fieldNames ...string) error {
	for _, fieldName := range fieldNames {
		if _, ok := d.GetOk(fieldName); !ok {
			return fmt.Errorf("%q is required when type = %q", fieldName, channelType)
		}
	}

	return nil
}

func setPrometheusNotificationChannelSecretIfReturned(d *schema.ResourceData, key, value string) {
	if value == "" {
		return
	}

	d.Set(key, value)
}
