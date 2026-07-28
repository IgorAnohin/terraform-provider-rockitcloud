terraform {
  required_version = ">= 1.3.0"

  required_providers {
    aws = {
      source = "c2devel/rockitcloud"
    }
  }
}

provider "aws" {
  region = var.region
}

