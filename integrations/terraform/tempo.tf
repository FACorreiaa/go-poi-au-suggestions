# Tempo PersistentVolumeClaim
resource "kubernetes_persistent_volume_claim" "tempo_pvc" {
  metadata {
    name      = "tempo-pvc"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "5Gi"
      }
    }
  }
}

# Tempo Deployment
resource "kubernetes_deployment" "tempo" {
  metadata {
    name      = "tempo"
    namespace = kubernetes_namespace.observability.metadata[0].name
    labels = {
      app = "tempo"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "tempo"
      }
    }
    template {
      metadata {
        labels = {
          app = "tempo"
        }
      }
      spec {
        container {
          image = "grafana/tempo:latest"
          name  = "tempo"
          
          args = ["-config.file=/etc/tempo.yaml"]

          port {
            container_port = 3200
            name          = "http"
          }

          port {
            container_port = 4317
            name          = "otlp-grpc"
          }

          port {
            container_port = 4318
            name          = "otlp-http"
          }

          volume_mount {
            name       = "tempo-config"
            mount_path = "/etc"
          }

          volume_mount {
            name       = "tempo-storage"
            mount_path = "/var/tempo"
          }

          resources {
            limits = {
              memory = "1Gi"
              cpu    = "500m"
            }
            requests = {
              memory = "512Mi"
              cpu    = "250m"
            }
          }

          liveness_probe {
            http_get {
              path = "/ready"
              port = 3200
            }
            initial_delay_seconds = 45
          }

          readiness_probe {
            http_get {
              path = "/ready"
              port = 3200
            }
            initial_delay_seconds = 45
          }
        }
        
        volume {
          name = "tempo-config"
          config_map {
            name = kubernetes_config_map.tempo_config.metadata[0].name
          }
        }

        volume {
          name = "tempo-storage"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.tempo_pvc.metadata[0].name
          }
        }
      }
    }
  }
}

# Tempo Service
resource "kubernetes_service" "tempo" {
  metadata {
    name      = "tempo-service"
    namespace = kubernetes_namespace.observability.metadata[0].name
  }
  spec {
    selector = {
      app = "tempo"
    }
    port {
      port        = 3200
      target_port = 3200
      name        = "http"
    }
    port {
      port        = 4317
      target_port = 4317
      name        = "otlp-grpc"
    }
    port {
      port        = 4318
      target_port = 4318
      name        = "otlp-http"
    }
    type = "ClusterIP"
  }
}