package docs

import (
	"encoding/json"
	"net/http"
)

// OpenAPISpec represents a simplified OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI string                 `json:"openapi"`
	Info    Info                   `json:"info"`
	Servers []Server               `json:"servers"`
	Paths   map[string]interface{} `json:"paths"`
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

var spec OpenAPISpec

func init() {
	spec = OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "Auth Service API",
			Description: "Authentication and authorization service API",
			Version:     "1.0.0",
		},
		Servers: []Server{
			{URL: "http://localhost:8080", Description: "Local development server"},
		},
		Paths: map[string]interface{}{
			"/auth/v1/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health Check",
					"description": "Check the health status of the service and its dependencies",
					"operationId": "healthCheck",
					"tags":        []string{"Health"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Service is healthy",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
									},
								},
							},
						},
						"503": map[string]interface{}{
							"description": "Service is unhealthy",
						},
					},
				},
			},
			"/auth/v1/register": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Register User",
					"description": "Register a new user account",
					"operationId": "register",
					"tags":        []string{"Authentication"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email", "password", "username"},
									"properties": map[string]interface{}{
										"email": map[string]interface{}{
											"type":    "string",
											"format":  "email",
											"example": "user@example.com",
										},
										"password": map[string]interface{}{
											"type":      "string",
											"minLength": 8,
											"example":   "SecurePass123",
										},
										"username": map[string]interface{}{
											"type":      "string",
											"minLength": 3,
											"maxLength": 50,
											"example":   "johndoe",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{
							"description": "User registered successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid input",
						},
						"409": map[string]interface{}{
							"description": "User already exists",
						},
					},
				},
			},
			"/auth/v1/login": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Login",
					"description": "Authenticate user and receive access/refresh tokens",
					"operationId": "login",
					"tags":        []string{"Authentication"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"email", "password"},
									"properties": map[string]interface{}{
										"email": map[string]interface{}{
											"type":   "string",
											"format": "email",
										},
										"password": map[string]interface{}{
											"type": "string",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Login successful",
						},
						"401": map[string]interface{}{
							"description": "Invalid credentials",
						},
					},
				},
			},
			"/auth/v1/me": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Get Current User",
					"description": "Get the current authenticated user's information",
					"operationId": "getCurrentUser",
					"tags":        []string{"User"},
					"security": []map[string]interface{}{
						{"BearerAuth": []string{}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "User information",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
		},
	}
}

// OpenAPIHandler returns the OpenAPI specification as JSON
func OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(spec)
}
