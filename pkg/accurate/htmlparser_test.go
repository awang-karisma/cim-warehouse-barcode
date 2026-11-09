package accurate_test

import (
	"strings"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestExtractForm(t *testing.T) {
	htmlContent := `
		<!DOCTYPE html>
		<html>
		<head><title>Test SSO</title></head>
		<body>
			<form action="https:&#x2f;&#x2f;account.accurate.id&#x2f;idp&#x2f;sso" method="post">
				<input type="hidden" name="SAMLRequest" value="ENCODED_SAML_REQUEST" />
				<input type="hidden" name="RelayState" value="https:&#x2f;&#x2f;zeus.accurate.id&#x2f;accurate" />
			</form>
		</body>
		</html>
	`

	form, err := accurate.ExtractForm(strings.NewReader(htmlContent), "SAMLRequest")
	if err != nil {
		t.Fatalf("ExtractForm failed: %v", err)
	}

	if form.Action != "https://account.accurate.id/idp/sso" {
		t.Fatalf("Action not unescaped properly: %s", form.Action)
	}

	if form.Fields["SAMLRequest"] != "ENCODED_SAML_REQUEST" {
		t.Fatalf("Field SAMLRequest mismatch: %s", form.Fields["SAMLRequest"])
	}

	if form.Fields["RelayState"] != "https://zeus.accurate.id/accurate" {
		t.Fatalf("Field RelayState mismatch: %s", form.Fields["RelayState"])
	}
}

func TestExtractContentDataFromScripts(t *testing.T) {
	htmlContent := `
		<!DOCTYPE html>
		<html>
		<head><title>Manage</title></head>
		<body>
			<script type="text/javascript">
				var randomVar = 123;
				var contentData = {
					"dbList": [
						{
							"id": 3026472,
							"alias": "Retail Demo",
							"host": "https://zeus.accurate.id",
							"uniqueId": "727792fa-9156-462f-a31a-b2d61b7a27f9",
							"isPrivateCloud": false
						}
					],
					"user": {
						"name": "Karisma User"
					}
				};
				console.log(contentData);
			</script>
		</body>
		</html>
	`

	data, err := accurate.ExtractContentDataFromScripts(strings.NewReader(htmlContent))
	if err != nil {
		t.Fatalf("ExtractContentDataFromScripts failed: %v", err)
	}

	dbList, ok := data["dbList"].([]any)
	if !ok || len(dbList) != 1 {
		t.Fatalf("dbList parsing failed: %+v", data["dbList"])
	}

	firstDB := dbList[0].(map[string]any)
	if firstDB["alias"] != "Retail Demo" {
		t.Fatalf("Alias mismatch: %v", firstDB["alias"])
	}
}
