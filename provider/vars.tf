variable "endpoint" {
  description = "The controller IP or Hostname."
  type        = string
  default     = "10.0.0.1" # Example default
}

variable "username" {
  description = "Username for basic auth."
  type        = string
  default     = "admin"
}

variable "password" {
  description = "Password for basic auth."
  type        = string
  sensitive   = true
  default     = ""
}

variable "token" {
  description = "Bearer token (mutually exclusive with username/password)."
  type        = string
  sensitive   = true
  default     = ""
}

variable "insecure" {
  description = "Skip TLS verification."
  type        = bool
  default     = true
}

variable "pool_id" {
  description = "The Storage Pool ID (Ref) to provision volumes/repositories in."
  type        = string
  default     = "04000000600A098000E3C1B000002CED62CF874D"
}

variable "host_port_type" {
  description = "The storage protocol used by host 1 (iscsi, nvmeof)."
  type        = string
  default     = "nvmeof"
}

variable "santricity_host_port_type" {
  description = "The storage protocol used by cluster members (iscsi or nvmeof)."
  type        = string
  default     = "nvmeof"
}

variable "santricity_host_1_port_identifier" {
  description = "The unique port identifier (IQN for iSCSI, NQN for NVMe/RoCE) used by host 1"
  type        = string
  default     = ""
}


variable "santricity_host_2_port_identifier" {
  description = "The unique port identifier (IQN for iSCSI, NQN for NVMe/RoCE) used by host 2"
  type        = string
  default     = ""
}

