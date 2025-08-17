package example

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Module provides example implementations
var Module = fx.Module("example",
	fx.Provide(
		NewUserHandler,
		NewOrderHandler,
		NewPaymentHandler,
		NewAMQPExample,
		NewCronExample,
		NewRepositoryExample,
		NewSimpleDemoHandler,
		NewEnhancedDemoHandler,
	),
	fx.Invoke(
		RegisterRoutes,
		StartBackgroundWorkers,
		RegisterSimpleDemo,
		RegisterEnhancedDemo,
	),
)

// RegisterSimpleDemo registers simple API demo routes
func RegisterSimpleDemo(router *gin.Engine, handler *SimpleDemoHandler) {
	RegisterSimpleRoutes(router, handler)
}

// RegisterEnhancedDemo is now defined in enhanced_demo.go
