// Command openapi generates and validates the OpenAPI contract from handler annotations.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/swaggo/swag"
	"sigs.k8s.io/yaml"
)

func main() {
	if err := generate(); err != nil {
		log.Fatal(err)
	}
}

func generate() error {
	parser := swag.New(swag.SetStrict(true))
	parser.ParseInternal = true
	if err := parser.ParseAPIMultiSearchDir([]string{"cmd/server", "internal/server", "internal/monitor"}, "main.go", 100); err != nil {
		return fmt.Errorf("parse Swagger annotations: %w", err)
	}
	data, err := json.Marshal(parser.GetSwagger())
	if err != nil {
		return err
	}
	var swagger openapi2.T
	if err := json.Unmarshal(data, &swagger); err != nil {
		return err
	}
	doc, err := openapi2conv.ToV3(&swagger)
	if err != nil {
		return fmt.Errorf("convert to OpenAPI 3: %w", err)
	}
	// Swagger 2 annotations cannot express HTTP bearer auth or cookie parameters.
	doc.Components.SecuritySchemes["BearerAuth"].Value = &openapi3.SecurityScheme{
		Type: "http", Scheme: "bearer", BearerFormat: "JWT",
		Description: "JWT access token, полученный при регистрации, входе или обновлении токенов.",
	}
	cookie := doc.Components.SecuritySchemes["RefreshCookie"].Value
	cookie.In, cookie.Name = "cookie", "refresh_token"
	for _, path := range doc.Paths.Map() {
		for _, operation := range path.Operations() {
			// Conversion keeps required form fields but loses the required body flag.
			if operation.RequestBody != nil {
				body := operation.RequestBody.Value
				if form := body.Content.Get("multipart/form-data"); form != nil && len(form.Schema.Value.Required) > 0 {
					body.Required = true
				}
			}
			for _, parameter := range operation.Parameters {
				if parameter.Value.In == "header" && parameter.Value.Name == "Cookie" {
					parameter.Value.In, parameter.Value.Name = "cookie", "refresh_token"
				}
			}
		}
	}
	// FileServer returns plain text for missing files, independently of image types.
	missingAvatar := doc.Paths.Value("/uploads/avatars/{filename}").Get.Responses.Value("404").Value
	missingAvatar.Content = openapi3.NewContentWithSchema(openapi3.NewStringSchema(), []string{"text/plain"})
	if err := doc.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate OpenAPI contract: %w", err)
	}
	data, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	yamlData, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("docs", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile("docs/openapi.json", append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile("docs/openapi.yaml", yamlData, 0o644)
}
