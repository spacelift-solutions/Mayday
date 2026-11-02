terraform {
  required_version = ">= 1.11.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" { region = var.region }

variable "region" {
  type    = string
  default = "us-east-1"
}
variable "license_bucket" { type = string }
variable "request_id" {
  type = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9-]+$", var.request_id))
    error_message = "Use a deployment identifier containing letters, numbers, and hyphens."
  }
}
variable "license_key" {
  type        = string
  description = "The attached plan policy permits only DEMO-ONLY."
}

locals {
  content = "${jsonencode({ service = "mayday", license_key = var.license_key, enabled = true })}\n"
}

# Keep template state separate from Intent's live license object.
resource "aws_s3_object" "request" {
  bucket                 = var.license_bucket
  key                    = "license-requests/${var.request_id}.json"
  content                = local.content
  content_type           = "application/json"
  etag                   = md5(local.content)
  server_side_encryption = "AES256"
}

output "license_request_uri" { value = "s3://${aws_s3_object.request.bucket}/${aws_s3_object.request.key}" }
output "license_content" { value = local.content }
