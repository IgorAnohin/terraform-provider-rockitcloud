resource "aws_paas_service" "elk" {
  name          = var.service_name
  instance_type = var.instance_type

  high_availability   = var.high_availability
  arbitrator_required = var.arbitrator_required

  root_volume {
    type = "gp2"
    size = 32
  }

  data_volume {
    type = "gp2"
    size = 32
  }

  delete_interfaces_on_destroy = true
  security_group_ids           = var.security_group_ids
  subnet_ids                   = var.subnet_ids
  ssh_key_name                 = var.ssh_key_name

  elk {
    version         = var.elk_version
    password        = var.elk_password
    allow_anonymous = var.allow_anonymous
    anonymous_role  = var.allow_anonymous == true ? var.anonymous_roles : null
    options         = var.elk_options

    dynamic "monitoring" {
      for_each = var.monitoring_service_id == null ? [] : [var.monitoring_service_id]

      content {
        monitor_by        = monitoring.value
        monitoring_labels = var.monitoring_labels
      }
    }
  }
}

data "aws_paas_service" "elk" {
  id = aws_paas_service.elk.id
}

resource "aws_paas_logstash_pipeline" "qa" {
  service_id    = aws_paas_service.elk.id
  name          = var.pipeline_name
  configuration = var.pipeline_configuration
}

resource "aws_paas_logstash_pipeline" "wrong_service" {
  count = var.wrong_service_id == null ? 0 : 1

  service_id    = var.wrong_service_id
  name          = "${var.pipeline_name}-invalid-target"
  configuration = "input { stdin {} }"
}
