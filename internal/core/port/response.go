package port

import "github.com/gin-gonic/gin"

// ResponseBuilder is the minimal interface used by core options/trace
type ResponseBuilder interface {
	Msg(string)
	Body(interface{})
	Status(string)
	Detail(string)
	Prt(bool)
	Err(error) error
	Send()
}

// ResponseBuilderFactory constructs a builder bound to Gin and status
type ResponseBuilderFactory interface {
	New(g *gin.Context, printer interface {
		Prt(format string, args ...interface{})
		GetTraceID() string
	}, status int) ResponseBuilder
}
