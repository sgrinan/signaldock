resource "aws_secretsmanager_secret" "signaldock" {
  name = "${var.project_name}/${var.environment}/signaldock"

  description = "SignalDock application credentials"
}

resource "aws_secretsmanager_secret" "postgres" {
  name = "${var.project_name}/${var.environment}/postgres"

  description = "SignalDock PostgreSQL credentials"
}

resource "aws_iam_role" "external_secrets" {
  name = "${var.project_name}-${var.environment}-external-secrets"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"

    Statement = [
      {
        Effect = "Allow"

        Principal = {
          Service = "pods.eks.amazonaws.com"
        }

        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
      }
    ]
  })
}

resource "aws_iam_policy" "external_secrets" {
  name = "${var.project_name}-${var.environment}-external-secrets"

  policy = jsonencode({
    Version = "2012-10-17"

    Statement = [
      {
        Effect = "Allow"

        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]

        Resource = [
          aws_secretsmanager_secret.signaldock.arn,
          aws_secretsmanager_secret.postgres.arn
        ]
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "external_secrets" {
  role       = aws_iam_role.external_secrets.name
  policy_arn = aws_iam_policy.external_secrets.arn
}

resource "aws_eks_pod_identity_association" "external_secrets" {
  cluster_name    = aws_eks_cluster.main.name
  namespace       = "external-secrets"
  service_account = "external-secrets"
  role_arn        = aws_iam_role.external_secrets.arn

  depends_on = [
    aws_eks_addon.pod_identity_agent
  ]
}