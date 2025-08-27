// @title Document AI API
// @version 1.0
// @description API для загрузки и обработки документов через n8n
// @host localhost:8080
// @BasePath /

package main

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	_ "document-ai/docs"
)

func main() {
	initEnvVariables()
	setupRoutes()

	// Swagger endpoint
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	startServer()
}
