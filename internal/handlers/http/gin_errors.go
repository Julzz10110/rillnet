package http

import "github.com/gin-gonic/gin"

// reportError attaches an error for ErrorHandlerMiddleware.
// gin.Context.Error returns *gin.Error which errcheck expects to handle.
func reportError(c *gin.Context, err error) {
	_ = c.Error(err)
}
