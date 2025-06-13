variable "app_name" {
  description = "Application name"
  type        = string
  default     = "loci-server"
}

variable "app_version" {
  description = "Application version"
  type        = string
  default     = "latest"
}

variable "app_replicas" {
  description = "Number of application replicas"
  type        = number
  default     = 2
}

variable "postgres_user" {
  description = "PostgreSQL username"
  type        = string
  default     = "postgres"
}

variable "postgres_password" {
  description = "PostgreSQL password"
  type        = string
  sensitive   = true
  default     = "postgres"
}

variable "postgres_db" {
  description = "PostgreSQL database name"
  type        = string
  default     = "loci-dev"
}

variable "jwt_secret" {
  description = "JWT secret key"
  type        = string
  sensitive   = true
  default     = "your-jwt-secret-key"
}

variable "jwt_issuer" {
  description = "JWT issuer"
  type        = string
  default     = "loci-server"
}

variable "jwt_audience" {
  description = "JWT audience"
  type        = string
  default     = "loci-server-users"
}

variable "domain" {
  description = "Domain for ingress"
  type        = string
  default     = "loci-server.local"
}

variable "enable_ingress" {
  description = "Enable ingress controller"
  type        = bool
  default     = true
}