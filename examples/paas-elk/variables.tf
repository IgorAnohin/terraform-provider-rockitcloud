variable "region" {
  type        = string
  description = "K2 Cloud region."
  default     = "ru-msk"
}

variable "service_name" {
  type        = string
  description = "Name of the ELK service used by manual QA."
  default     = "tf-elk-qa"
}

variable "instance_type" {
  type        = string
  description = "Instance type for ELK nodes."
  default     = "c5.large"
}

variable "subnet_ids" {
  type        = set(string)
  description = "Internet-access subnet IDs for the ELK service."

  validation {
    condition     = length(var.subnet_ids) > 0
    error_message = "At least one subnet ID is required."
  }
}

variable "security_group_ids" {
  type        = set(string)
  description = "Security group IDs for the ELK service."

  validation {
    condition     = length(var.security_group_ids) > 0
    error_message = "At least one security group ID is required."
  }
}

variable "ssh_key_name" {
  type        = string
  description = "Existing K2 Cloud SSH key name."
}

variable "high_availability" {
  type        = bool
  description = "Whether to create a high-availability ELK service."
  default     = false
}

variable "arbitrator_required" {
  type        = bool
  description = "Whether the high-availability ELK service requires an arbitrator."
  default     = false
}

variable "elk_version" {
  type        = string
  description = "ELK version."
  default     = "8.17"
}

variable "elk_password" {
  type        = string
  description = "Elasticsearch user password."
  sensitive   = true
  default     = null
}

variable "allow_anonymous" {
  type        = bool
  description = "Whether anonymous Kibana access is enabled."
  default     = null
}

variable "anonymous_roles" {
  type        = set(string)
  description = "Roles assigned to anonymous Kibana users."
  default     = ["viewer"]

  validation {
    condition = alltrue([
      for role in var.anonymous_roles : contains(["viewer", "editor"], role)
    ])
    error_message = "Anonymous roles must be viewer or editor."
  }
}

variable "elk_options" {
  type        = map(string)
  description = "Additional ELK parameters."
  default     = {}
}

variable "monitoring_service_id" {
  type        = string
  description = "Prometheus service ID in the same VPC. Null disables monitoring."
  default     = null
}

variable "monitoring_labels" {
  type        = map(string)
  description = "Labels sent with ELK metrics."
  default = {
    environment = "qa"
    managed_by  = "terraform"
  }
}

variable "pipeline_name" {
  type        = string
  description = "Logstash pipeline name."
  default     = "terraform-qa"
}

variable "pipeline_configuration" {
  type        = string
  description = "Logstash pipeline configuration."
  sensitive   = true
  default     = <<-LOGSTASH
    input {
      http {
        port => 4567
        tags => ["terraform", "qa", "first"]
      }
    }
  LOGSTASH
}

variable "wrong_service_id" {
  type        = string
  description = "Non-ELK PaaS service ID used only by the isolated negative QA case."
  default     = null
}
