package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/platform9/vjailbreak/ui-proxy/pkg/handlers"
	"github.com/platform9/vjailbreak/ui-proxy/pkg/kube"
	"github.com/platform9/vjailbreak/ui-proxy/pkg/utils"
)

func main() {
	kubeClient := kube.NewClient()
	server := handlers.NewProxyServer(kubeClient)

	router := gin.Default()

	// Middleware
	router.Use(utils.CORSMiddleware())

	// Endpoints
	router.GET("/health", utils.HealthHandler)
	router.POST("/proxy/openstack", server.HandleOpenStackProxy)
	router.POST("/proxy/vmware", server.HandleVMwareProxy)

	port := utils.GetEnv("PORT", "8080")
	log.Printf("Starting proxy server on port %s", port)
	log.Fatal(router.Run(":" + port))
}
