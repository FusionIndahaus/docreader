// @title Document AI API
// @version 1.0
// @description API для загрузки и обработки документов через n8n
// @host 45.82.153.200
// @BasePath /

package main

func main() {
	initEnvVariables()
	if err := initDatabase(); err != nil {
		println("WARNING: DB connect failed:", err.Error())
	}
	defer closeDatabase()
	setupRoutes()
	startServer()
}
