package port

// ResponseBuilder is the minimal interface used by core options/trace.
// The Gin-bound factory that used to accompany it was removed so the
// core ports carry no web-framework dependency; the supported Gin
// integration is the public boeng/gin package.
type ResponseBuilder interface {
	Msg(string)
	Body(interface{})
	Status(string)
	Detail(string)
	Prt(bool)
	Err(error) error
	Send()
}
