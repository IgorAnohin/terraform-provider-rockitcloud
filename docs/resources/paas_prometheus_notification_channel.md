---
subcategory: "PaaS"
layout: "aws"
page_title: "aws_paas_prometheus_notification_channel"
description: |-
  Manages a Prometheus notification channel for a PaaS service.
---

# Resource: aws_paas_prometheus_notification_channel

Manages a Prometheus notification channel for a PaaS service.

## Example Usage

### Telegram Channel

```terraform
resource "aws_paas_prometheus_notification_channel" "telegram" {
  service_id = aws_paas_service.prometheus.id

  name      = "ops-telegram"
  type      = "telegram"
  bot_token = var.telegram_bot_token
  chat_id   = -1001234567890
}
```

### Webhook Channel

```terraform
resource "aws_paas_prometheus_notification_channel" "webhook" {
  service_id = aws_paas_service.prometheus.id

  name       = "ops-webhook"
  type       = "webhook"
  url        = "https://alertmanager.example.com/hook"
  max_alerts = 10
}
```

### Email Channel

```terraform
resource "aws_paas_prometheus_notification_channel" "email" {
  service_id = aws_paas_service.prometheus.id

  name          = "ops-email"
  type          = "email"
  to            = "ops@example.com"
  from          = "alerts@example.com"
  smarthost     = "smtp.example.com:587"
  hello         = "example.com"
  require_tls   = true
  auth_username = "alerts@example.com"
  auth_password = var.smtp_password
}
```

## Argument Reference

The following arguments are supported:

* `service_id` - (Required, Forces new resource) The ID of the PaaS Prometheus service.
* `name` - (Required) The name of the notification channel. Must be between 1 and 256 characters.
* `type` - (Required) The channel type. Valid values: `telegram`, `webhook`, `email`.
* `is_default` - (Optional) Whether this channel is the default receiver. Defaults to `false`.
* `send_resolved` - (Optional) Whether to send notifications when an alert resolves. Defaults to `false`.

### Telegram arguments

The following arguments apply when `type = "telegram"`:

* `bot_token` - (Required) The Telegram bot token. Sensitive.
* `chat_id` - (Required) The Telegram chat ID. May be negative for group chats.

### Webhook arguments

The following arguments apply when `type = "webhook"`:

* `url` - (Required) The webhook URL. Must be between 1 and 2048 characters.
* `max_alerts` - (Optional) Maximum number of alerts per webhook request. `0` means no limit.

### Email arguments

The following arguments apply when `type = "email"`:

* `to` - (Required) The recipient address.
* `from` - (Required) The sender address.
* `smarthost` - (Required) The SMTP relay host and port (e.g. `smtp.example.com:587`).
* `hello` - (Optional) The hostname used in the SMTP `EHLO` command.
* `require_tls` - (Optional) Whether to require TLS. Defaults to `false`.
* `auth_username` - (Optional) The SMTP authentication username. Required together with `auth_password`.
* `auth_password` - (Optional) The SMTP authentication password. Sensitive. Required together with `auth_username`.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the notification channel.
* `channel_id` - The ID of the notification channel (same as `id`).

## Import

PaaS Prometheus notification channels can be imported using `service_id/channel_id`, e.g.,

```
$ terraform import aws_paas_prometheus_notification_channel.example fm-cluster-12345678/channel-87654321
```
