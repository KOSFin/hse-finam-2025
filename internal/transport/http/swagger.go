package transporthttp

import (
	"net/http"

	"finamhackbackend/docs"
)

var swaggerPage = []byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Radar API · Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html, body { margin: 0; padding: 0; height: 100%; }
    #swagger-ui { height: 100%; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.addEventListener('load', function() {
      SwaggerUIBundle({
        url: '/swagger/openapi.yaml',
        dom_id: '#swagger-ui'
      });
    });
  </script>
</body>
</html>`)

func serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if len(docs.OpenAPISpec) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(swaggerPage)
}

func serveSwaggerYAML(w http.ResponseWriter, r *http.Request) {
	if len(docs.OpenAPISpec) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docs.OpenAPISpec)
}
