output "elk_id" {
  value = aws_paas_service.elk.id
}

output "elk_status" {
  value = aws_paas_service.elk.status
}

output "elk_service_type" {
  value = aws_paas_service.elk.service_type
}

output "elk_service_class" {
  value = aws_paas_service.elk.service_class
}

output "elk_instances" {
  value = aws_paas_service.elk.instances
}

output "elk_endpoints" {
  value = aws_paas_service.elk.endpoints
}

output "elk_version_from_data_source" {
  value = data.aws_paas_service.elk.elk[0].version
}

output "pipeline_id" {
  value = aws_paas_logstash_pipeline.qa.pipeline_id
}

output "pipeline_name" {
  value = aws_paas_logstash_pipeline.qa.name
}

