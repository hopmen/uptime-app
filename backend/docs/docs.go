// Package docs embeds the generated OpenAPI contract in the server binary.
package docs

import "embed"

// Generate from the backend root so paths do not depend on the caller's directory.
//go:generate sh -c "cd .. && go run ./cmd/openapi"

// Files contains both downloadable representations of the API contract.
//
//go:embed openapi.json openapi.yaml
var Files embed.FS
