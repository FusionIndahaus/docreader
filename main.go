// @title Document AI API
// @version 1.0
// @description API для загрузки и обработки документов через n8n
// @host localhost:8080
// @BasePath /

package main

func main() {
	initEnvVariables()
	setupRoutes()
	startServer()
}
