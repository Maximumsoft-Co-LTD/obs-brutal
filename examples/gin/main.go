// Gin adapter demo. Run:  go run ./examples/gin
//
// Open http://localhost:8080/users/u-7 in a second terminal:
//   curl http://localhost:8080/users/u-7
//
// Notice that the handler never imports OpenTelemetry. boenggin.Middleware
// opens a boeng operation per request and boenggin.L(c) hands the
// handler the request-scoped logger.
package main

import (
	"github.com/gin-gonic/gin"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boenggin "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/gin"
)

func main() {
	defer boeng.Init(boeng.Config{Service: "gin_demo", Env: "dev"}).Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(boenggin.Middleware())

	r.GET("/users/:id", func(c *gin.Context) {
		boenggin.L(c).F("user_id", c.Param("id")).Info("user fetch requested")
		c.JSON(200, gin.H{"id": c.Param("id"), "name": "demo"})
	})

	_ = r.Run(":8080")
}
