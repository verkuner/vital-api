package docs

import (
	"context"
	_ "embed"
	"net/http"

	encore "encore.dev"
)

//go:embed openapi.json
var openapiSpec []byte

//encore:service
type Service struct{}

func initService() (*Service, error) {
	return &Service{}, nil
}

type SpecResponse struct {
	Spec string `json:"spec" encore:"raw"`
}

//encore:api public method=GET path=/docs/openapi.json
func (s *Service) OpenAPISpec(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(openapiSpec)
}

//encore:api public raw method=GET path=/docs
func (s *Service) SwaggerUI(w http.ResponseWriter, req *http.Request) {
	env := encore.Meta().Environment.Name
	baseURL := "http://127.0.0.1:5080"
	if env != "" && env != "local" {
		baseURL = "https://staging-vital-api-cq4i.encr.app"
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Vital Signs API — Documentation</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "` + baseURL + `/docs/openapi.json",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIBundle.SwaggerUIStandalonePreset
      ],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

//encore:api public method=GET path=/docs/health
func (s *Service) Health(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "docs"}, nil
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}
