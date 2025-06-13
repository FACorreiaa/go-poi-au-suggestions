# Application secrets
resource "kubernetes_secret" "app_secrets" {
  metadata {
    name      = "loci-server-secrets"
    namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
  }

  type = "Opaque"

  data = {
    JWT_SECRET_KEY = base64encode(var.jwt_secret)
    JWT_ISSUER     = base64encode(var.jwt_issuer)
    JWT_AUDIENCE   = base64encode(var.jwt_audience)
  }
}

# PostgreSQL secrets
resource "kubernetes_secret" "postgres_secrets" {
  metadata {
    name      = "postgres-secrets"
    namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
  }

  type = "Opaque"

  data = {
    POSTGRES_USER     = base64encode(var.postgres_user)
    POSTGRES_PASSWORD = base64encode(var.postgres_password)
    POSTGRES_DB       = base64encode(var.postgres_db)
  }
}

# Grafana admin secrets
resource "kubernetes_secret" "grafana_secrets" {
  metadata {
    name      = "grafana-secrets"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }

  type = "Opaque"

  data = {
    admin-user     = base64encode("admin")
    admin-password = base64encode("admin123")
  }
}