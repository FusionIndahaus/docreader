// @title Document AI API
// @version 1.0
// @description API для загрузки и обработки документов через n8n
// @host 45.82.153.200
// @BasePath /

package main

func main() {
	initEnvVariables()
	setupRoutes()
	startServer()
}
