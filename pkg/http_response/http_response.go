package http_response

import "github.com/gin-gonic/gin"

// Error response
func ErrorResponse(statusCode int, message string) gin.H {
	return gin.H{
		"statusCode": statusCode,
		"message":    message,
		"data":       nil,
	}
}

// Success response
func SuccessResponse(data interface{}, message string) gin.H {
	return gin.H{
		"statusCode": 200,
		"message":    message,
		"data":       data,
	}
}

// Created response
func CreatedResponse(data interface{}, message string) gin.H {
	return gin.H{
		"statusCode": 201,
		"message":    message,
		"data":       data,
	}
}
