terraform {
  required_providers {
    flows = {
      source  = "spacelift-io/flows"
      version = ">= 0.6.0, < 1.0.0"
    }
  }
}
provider "flows" { endpoint = var.flows_endpoint }
variable "flows_endpoint" {
  type    = string
  default = "useflows.eu"
}
variable "project_id" { type = string }
variable "app_version_id" { type = string }
variable "api_key_id" { type = string }
variable "api_key_secret" {
  type      = string
  sensitive = true
}
variable "webhook_token" {
  type      = string
  sensitive = true
}

resource "flows_app_installation" "mayday" {
  project_id = var.project_id
  name       = "Mayday Monitor"
  app = {
    version_id = var.app_version_id
    custom     = true
  }
  confirm = false
}
locals {
  app_config = {
    webhookToken       = var.webhook_token
    spaceliftEndpoint  = "https://spacelift-solutions.app.spacelift.io"
    apiKeyID           = var.api_key_id
    apiKeySecret       = var.api_key_secret
    serviceStackID     = "mayday-service"
    diagnosticsStackID = "mayday-diagnostics"
  }
}
resource "flows_app_installation_config_field" "mayday" {
  for_each            = toset(["webhookToken", "spaceliftEndpoint", "apiKeyID", "apiKeySecret", "serviceStackID", "diagnosticsStackID"])
  app_installation_id = flows_app_installation.mayday.id
  key                 = each.key
  value               = jsonencode(local.app_config[each.key])
}
resource "flows_app_installation_confirmation" "mayday" {
  app_installation_id = flows_app_installation.mayday.id
  depends_on          = [flows_app_installation_config_field.mayday]
}

resource "flows_flow" "recovery" {
  project_id               = var.project_id
  name                     = "Mayday recovery"
  definition               = file("${path.module}/../recovery.yaml")
  app_installation_mapping = { mayday = flows_app_installation.mayday.id }
  depends_on               = [flows_app_installation_confirmation.mayday]
}
output "flow_id" { value = flows_flow.recovery.id }
