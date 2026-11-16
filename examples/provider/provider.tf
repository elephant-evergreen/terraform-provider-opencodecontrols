terraform {
  required_providers {
    opencodecontrols = {
      source  = "elephant-evergreen/opencodecontrols"
      version = "= 0.0.1-alpha.1"
    }
  }
}

provider "opencodecontrols" {}
