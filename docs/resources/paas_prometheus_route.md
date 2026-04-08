---
subcategory: "PaaS"
layout: "aws"
page_title: "aws_paas_prometheus_route"
description: |-
  Manages a Prometheus alerting route for a PaaS service.
---

# Resource: aws_paas_prometheus_route

Manages a Prometheus alerting route for a PaaS service. Routes define how incoming alerts are matched and forwarded to notification channels.

## Example Usage

```terraform
resource "aws_paas_prometheus_notification_channel" "telegram" {
  service_id = aws_paas_service.prometheus.id
  name       = "ops-telegram"
  type       = "telegram"
  bot_token  = var.telegram_bot_token
  chat_id    = -1001234567890
}

resource "aws_paas_prometheus_route" "critical" {
  service_id = aws_paas_service.prometheus.id

  name     = "critical-alerts"
  receiver = aws_paas_prometheus_notification_channel.telegram.channel_id
  matchers = ["severity=critical"]

  group_by        = ["alertname", "instance"]
  group_wait      = "30s"
  group_interval  = "5m"
  repeat_interval = "3h"
}
```

## Argument Reference

The following arguments are supported:

* `service_id` - (Required, Forces new resource) The ID of the PaaS Prometheus service.
* `name` - (Required) The name of the route. Must be between 1 and 256 characters.
* `receiver` - (Required) The ID of the notification channel that receives alerts matched by this route. Must be between 1 and 256 characters.
* `matchers` - (Optional) A set of label matchers that determine which alerts this route handles (e.g. `severity=critical`).
* `continue` - (Optional) Whether to continue matching subsequent routes after this one matches. Defaults to `false`.
* `group_by` - (Optional) A set of label names to group alerts by.
* `group_wait` - (Optional) How long to wait before sending the first notification for a new alert group (e.g. `30s`).
* `group_interval` - (Optional) How long to wait before sending a notification for new alerts added to an existing group (e.g. `5m`).
* `repeat_interval` - (Optional) How long to wait before re-sending a notification for an alert that has already been sent (e.g. `3h`).

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the route.
* `route_id` - The ID of the route (same as `id`).

## Import

PaaS Prometheus routes can be imported using `service_id/route_id`, e.g.,

```
$ terraform import aws_paas_prometheus_route.example fm-cluster-12345678/route-87654321
```
