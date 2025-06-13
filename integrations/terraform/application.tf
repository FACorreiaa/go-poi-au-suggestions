# Go AI POI Application Deployment
resource "kubernetes_deployment" "go_ai_poi_app" {
  metadata {
    name      = var.app_name
    namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
    labels = {
      app = var.app_name
    }
  }

  spec {
    replicas = var.app_replicas
    selector {
      match_labels = {
        app = var.app_name
      }
    }
    template {
      metadata {
        labels = {
          app = var.app_name
        }
        annotations = {
          "prometheus.io/scrape" = "true"
          "prometheus.io/port"   = "8084"
          "prometheus.io/path"   = "/metrics"
        }
      }
      spec {
        container {
          image = "${var.app_name}:${var.app_version}"
          name  = var.app_name
          
          # Main API port
          port {
            container_port = 8000
            name          = "http-api"
          }

          # External API port
          port {
            container_port = 8081
            name          = "external-api"
          }

          # Internal API port
          port {
            container_port = 8083
            name          = "internal-api"
          }

          # Metrics port
          port {
            container_port = 8084
            name          = "metrics"
          }

          # pprof port
          port {
            container_port = 8082
            name          = "pprof"
          }

          env {
            name  = "APP_ENV"
            value = "production"
          }

          env {
            name  = "LOG_LEVEL"
            value = "info"
          }

          env {
            name  = "OTEL_SERVICE_NAME"
            value = var.app_name
          }

          env {
            name  = "OTEL_EXPORTER_OTLP_ENDPOINT"
            value = "http://otel-collector-service.observability.svc.cluster.local:4317"
          }

          env {
            name = "JWT_SECRET_KEY"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.app_secrets.metadata[0].name
                key  = "JWT_SECRET_KEY"
              }
            }
          }

          env {
            name = "JWT_ISSUER"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.app_secrets.metadata[0].name
                key  = "JWT_ISSUER"
              }
            }
          }

          env {
            name = "JWT_AUDIENCE"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.app_secrets.metadata[0].name
                key  = "JWT_AUDIENCE"
              }
            }
          }

          volume_mount {
            name       = "app-config"
            mount_path = "/app/config"
          }

          resources {
            limits = {
              memory = "2Gi"
              cpu    = "1000m"
            }
            requests = {
              memory = "1Gi"
              cpu    = "500m"
            }
          }

          liveness_probe {
            http_get {
              path = "/health"
              port = 8000
            }
            initial_delay_seconds = 30
            period_seconds        = 10
            timeout_seconds       = 5
            failure_threshold     = 3
          }

          readiness_probe {
            http_get {
              path = "/ready"
              port = 8000
            }
            initial_delay_seconds = 30
            period_seconds        = 5
            timeout_seconds       = 3
            failure_threshold     = 3
          }
        }
        
        volume {
          name = "app-config"
          config_map {
            name = kubernetes_config_map.app_config.metadata[0].name
          }
        }
      }
    }
  }
}

# Go AI POI Application Service
resource "kubernetes_service" "go_ai_poi_app" {
  metadata {
    name      = "loci-server-service"
    namespace = kubernetes_namespace.go_ai_poi.metadata[0].name
    annotations = {
      "prometheus.io/scrape" = "true"
      "prometheus.io/port"   = "8084"
      "prometheus.io/path"   = "/metrics"
    }
  }
  spec {
    selector = {
      app = var.app_name
    }
    port {
      port        = 8000
      target_port = 8000
      name        = "http-api"
    }
    port {
      port        = 8081
      target_port = 8081
      name        = "external-api"
    }
    port {
      port        = 8083
      target_port = 8083
      name        = "internal-api"
    }
    port {
      port        = 8084
      target_port = 8084
      name        = "metrics"
    }
    port {
      port        = 8082
      target_port = 8082
      name        = "pprof"
    }
    type = "ClusterIP"
  }
}