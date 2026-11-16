package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	fixtureAccount = "svcacct_01AAAAAAAAAAAAAAAAAAAAAAAA"
	fixtureKey     = "sk_11111111-1111-4111-8111-111111111111"
	fixtureToken   = "oc_sk_fixture_management_value"
)

func TestAPIKeyLifecycle(t *testing.T) {
	t.Setenv("OPENCODE_CONTROLS_API_KEY", fixtureToken)
	var mu sync.Mutex
	status := ""
	creates := 0
	revocations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == accountsPath+fixtureAccount:
			keys := []keyRecord{}
			if status != "" {
				keys = append(keys, keyRecord{ID: fixtureKey, Name: "fixture", Permissions: "restricted", Scopes: []string{"models:read"}, Status: status})
			}
			json.NewEncoder(w).Encode(map[string]any{"account": map[string]string{"id": fixtureAccount}, "keys": keys})
		case r.Method == http.MethodPost && r.URL.Path == accountsPath+fixtureAccount+"/keys":
			var input createKeyRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name != "fixture" || input.Permissions != "restricted" || len(input.Scopes) != 1 || input.Scopes[0] != "models:read" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			creates++
			status = "active"
			json.NewEncoder(w).Encode(map[string]any{"key": map[string]string{"id": fixtureKey}, "token": "oc_sk_fixture_one_time_value"})
		case r.Method == http.MethodPost && r.URL.Path == accountsPath+"keys/"+fixtureKey+"/revoke":
			revocations++
			status = "revoked"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := `
resource "opencodecontrols_service_account_api_key" "test" {
  service_account_id = "` + fixtureAccount + `"
  name               = "fixture"
  scopes             = ["models:read"]
}
data "opencodecontrols_service_account_api_keys" "test" {
  service_account_id = opencodecontrols_service_account_api_key.test.service_account_id
  depends_on         = [opencodecontrols_service_account_api_key.test]
}
`
	address := "opencodecontrols_service_account_api_key.test"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"opencodecontrols": providerserver.NewProtocol6WithError(&controlsProvider{version: "test", origin: server.URL}),
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(address, "id", fixtureKey),
					resource.TestCheckResourceAttr(address, "status", "active"),
					resource.TestCheckResourceAttr(address, "value", "oc_sk_fixture_one_time_value"),
					resource.TestCheckResourceAttr("data.opencodecontrols_service_account_api_keys.test", "keys.0.id", fixtureKey),
				),
			},
			{
				ResourceName: address, ImportState: true, ImportStateVerify: true,
				ImportStateId:           fixtureAccount + "/" + fixtureKey,
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if creates != 1 || revocations != 1 || status != "revoked" {
		t.Fatal("Expected one creation and one revocation across the lifecycle and import.")
	}
}
