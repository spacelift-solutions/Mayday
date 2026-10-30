variable "region" {
  type    = string
  default = "us-east-1"
}
variable "cluster_name" {
  type = string
}
variable "ssh_public_key_file" {
  description = "Public key mounted by the Mayday admin stack"
  type        = string
  default     = "/mnt/workspace/mayday.pub"
}
variable "instance_type" {
  type    = string
  default = "t3.small"
}
variable "license_key" {
  type    = string
  default = "license/demo-license.json"
}
