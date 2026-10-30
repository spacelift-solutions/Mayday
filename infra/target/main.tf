data "aws_caller_identity" "current" {}
data "aws_eks_cluster" "shared" { name = var.cluster_name }
data "aws_vpc" "shared" { id = data.aws_eks_cluster.shared.vpc_config[0].vpc_id }
data "aws_ssm_parameter" "ubuntu" {
  name = "/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id"
}

locals {
  bucket_name = "mayday-${data.aws_caller_identity.current.account_id}-${var.region}"
  object_arn  = "${aws_s3_bucket.license.arn}/${var.license_key}"
}

resource "aws_s3_bucket" "license" {
  bucket        = local.bucket_name
  force_destroy = false
}
resource "aws_s3_bucket_public_access_block" "license" {
  bucket                  = aws_s3_bucket.license.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
resource "aws_s3_bucket_server_side_encryption_configuration" "license" {
  bucket = aws_s3_bucket.license.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}
# The object itself belongs to Intent. Do not add an aws_s3_object here.

resource "aws_key_pair" "target" {
  key_name   = "mayday"
  public_key = trimspace(file(var.ssh_public_key_file))
}
resource "aws_security_group" "target" {
  name_prefix = "mayday-"
  description = "Mayday target access from the shared VPC"
  vpc_id      = data.aws_vpc.shared.id
}
resource "aws_vpc_security_group_ingress_rule" "http" {
  security_group_id = aws_security_group.target.id
  from_port         = 8081
  to_port           = 8081
  ip_protocol       = "tcp"
  cidr_ipv4         = data.aws_vpc.shared.cidr_block
}
resource "aws_vpc_security_group_egress_rule" "target" {
  security_group_id = aws_security_group.target.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}
resource "aws_iam_role" "target" {
  name               = "mayday-target"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole" }] })
}
resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.target.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}
resource "aws_iam_role_policy" "license_read" {
  role = aws_iam_role.target.id
  policy = jsonencode({ Version = "2012-10-17", Statement = [
    { Effect = "Allow", Action = ["s3:GetObject"], Resource = local.object_arn },
    # ListBucket distinguishes missing objects from denied reads.
    { Effect = "Allow", Action = ["s3:ListBucket"], Resource = aws_s3_bucket.license.arn }
  ] })
}
resource "aws_iam_instance_profile" "target" {
  name = "mayday-target"
  role = aws_iam_role.target.name
}
resource "aws_instance" "target" {
  ami                         = data.aws_ssm_parameter.ubuntu.value
  instance_type               = var.instance_type
  subnet_id                   = sort(tolist(data.aws_eks_cluster.shared.vpc_config[0].subnet_ids))[0]
  associate_public_ip_address = false
  vpc_security_group_ids      = [aws_security_group.target.id]
  key_name                    = aws_key_pair.target.key_name
  iam_instance_profile        = aws_iam_instance_profile.target.name
  user_data                   = <<-SCRIPT
    #!/bin/bash
    set -euo pipefail
    snap list amazon-ssm-agent >/dev/null 2>&1 || snap install amazon-ssm-agent --classic
    systemctl enable --now snap.amazon-ssm-agent.amazon-ssm-agent.service
  SCRIPT
  metadata_options {
    http_tokens = "required"
  }
  root_block_device {
    encrypted   = true
    volume_size = 12
  }
  tags = { Name = "Mayday target" }
}
resource "aws_ssm_document" "stop" {
  name          = "MaydayStopService"
  document_type = "Command"
  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Deliberately stop only the Mayday target service"
    mainSteps     = [{ action = "aws:runShellScript", name = "stopMayday", inputs = { runCommand = ["systemctl stop mayday.service"], timeoutSeconds = "20" } }]
  })
}

resource "aws_iam_role" "monitor" {
  name = "mayday-monitor"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{
    Effect    = "Allow", Principal = { Service = "pods.eks.amazonaws.com" }, Action = ["sts:AssumeRole", "sts:TagSession"]
    Condition = { StringEquals = { "aws:SourceAccount" = data.aws_caller_identity.current.account_id }, ArnEquals = { "aws:SourceArn" = data.aws_eks_cluster.shared.arn } }
  }] })
}
resource "aws_iam_role_policy" "monitor" {
  role = aws_iam_role.monitor.id
  policy = jsonencode({ Version = "2012-10-17", Statement = [
    { Effect = "Allow", Action = ["s3:GetObject", "s3:PutObject"], Resource = local.object_arn },
    { Effect = "Allow", Action = ["ssm:SendCommand"], Resource = [aws_ssm_document.stop.arn, aws_instance.target.arn] }
  ] })
}
resource "aws_eks_pod_identity_association" "monitor" {
  cluster_name    = var.cluster_name
  namespace       = "mayday"
  service_account = "mayday-monitor"
  role_arn        = aws_iam_role.monitor.arn
}

module "tofusible_host" {
  source = "git::https://github.com/spacelift-solutions/tofusible.git//modules/tofusible_host?ref=463d0363148431da897c5fc864406024364d7884"

  host                 = aws_instance.target.id
  user                 = "ubuntu"
  ssh_private_key_file = "/mnt/workspace/mayday.pem"
  groups               = ["mayday"]
  extra_vars = {
    license_bucket          = aws_s3_bucket.license.id
    license_key             = var.license_key
    aws_region              = var.region
    ansible_ssh_common_args = "-o ProxyCommand=\"python3 ssm_proxy.py %h %p ${var.region}\""
  }
}

output "inventory_tofu" {
  # Mayday supplies host metadata and a key path, never passwords or key contents.
  value = [nonsensitive(module.tofusible_host.spec)]
}
output "region" { value = var.region }
output "target_url" { value = "http://${aws_instance.target.private_ip}:8081" }
output "instance_id" { value = aws_instance.target.id }
output "license_bucket" { value = aws_s3_bucket.license.id }
output "license_key" { value = var.license_key }
output "stop_document_name" { value = aws_ssm_document.stop.name }
output "cluster_name" { value = var.cluster_name }
output "pod_identity_association_id" { value = aws_eks_pod_identity_association.monitor.association_id }
