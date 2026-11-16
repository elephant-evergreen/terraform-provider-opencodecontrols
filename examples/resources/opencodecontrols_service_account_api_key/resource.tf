resource "opencodecontrols_service_account_api_key" "models" {
  service_account_id = "svcacct_01AAAAAAAAAAAAAAAAAAAAAAAA"
  name               = "model-catalog"
  scopes             = ["models:read"]
}
