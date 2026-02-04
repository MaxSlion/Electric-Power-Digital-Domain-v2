// Package docs contains the Swagger API documentation
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "termsOfService": "http://swagger.io/terms/",
        "contact": {
            "name": "API Support",
            "email": "support@example.com"
        },
        "license": {
            "name": "Apache 2.0",
            "url": "http://www.apache.org/licenses/LICENSE-2.0.html"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/api/v1/algorithms/schemes": {
            "get": {
                "description": "Returns a list of all registered algorithm schemes from the algorithm service",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["algorithms"],
                "summary": "Get available algorithm schemes",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/Scheme"
                            }
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/ErrorResponse"
                        }
                    }
                }
            }
        },
        "/api/v1/jobs": {
            "get": {
                "description": "Returns a paginated list of jobs with optional filters",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["jobs"],
                "summary": "List jobs with pagination",
                "parameters": [
                    {"type": "integer", "default": 1, "description": "Page number", "name": "page", "in": "query"},
                    {"type": "integer", "default": 20, "description": "Items per page", "name": "page_size", "in": "query"},
                    {"type": "string", "description": "Filter by user ID", "name": "user_id", "in": "query"},
                    {"type": "string", "description": "Filter by status", "name": "status", "in": "query", "enum": ["PENDING", "RUNNING", "SUCCESS", "FAILED"]}
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "jobs": {"type": "array", "items": {"$ref": "#/definitions/Job"}},
                                "total": {"type": "integer"},
                                "page": {"type": "integer"},
                                "page_size": {"type": "integer"},
                                "pages": {"type": "integer"}
                            }
                        }
                    }
                }
            },
            "post": {
                "description": "Creates a new job and dispatches it to the algorithm service for processing",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["jobs"],
                "summary": "Submit a new algorithm job",
                "parameters": [
                    {"type": "string", "description": "Idempotency key", "name": "X-Request-ID", "in": "header"},
                    {
                        "description": "Job submission request",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/SubmitJobRequest"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "job_id": {"type": "string"},
                                "status": {"type": "string"}
                            }
                        }
                    },
                    "400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "409": {"description": "Conflict - Duplicate request", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
        "/api/v1/jobs/{id}": {
            "get": {
                "description": "Returns detailed information about a specific job",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["jobs"],
                "summary": "Get job by ID",
                "parameters": [
                    {"type": "string", "description": "Job ID", "name": "id", "in": "path", "required": true}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"type": "object", "properties": {"job": {"$ref": "#/definitions/Job"}}}},
                    "404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
        "/api/v1/jobs/{id}/result": {
            "get": {
                "description": "Returns the result data for a completed job",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["jobs"],
                "summary": "Get job result",
                "parameters": [
                    {"type": "string", "description": "Job ID", "name": "id", "in": "path", "required": true}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"type": "object"}},
                    "400": {"description": "Job not completed", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
        "/api/v1/jobs/{id}/cancel": {
            "post": {
                "description": "Attempts to cancel a pending or running job",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["jobs"],
                "summary": "Cancel a running job",
                "parameters": [
                    {"type": "string", "description": "Job ID", "name": "id", "in": "path", "required": true}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/SuccessResponse"}},
                    "400": {"description": "Cannot cancel", "schema": {"$ref": "#/definitions/ErrorResponse"}},
                    "404": {"description": "Not Found", "schema": {"$ref": "#/definitions/ErrorResponse"}}
                }
            }
        },
        "/api/v1/system/health": {
            "get": {
                "description": "Returns the health status of the backend service and its dependencies",
                "produces": ["application/json"],
                "tags": ["system"],
                "summary": "Health check",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "status": {"type": "string", "enum": ["healthy", "degraded", "unhealthy"]},
                                "checks": {"type": "object"}
                            }
                        }
                    }
                }
            }
        },
        "/api/v1/system/stats": {
            "get": {
                "description": "Returns aggregate statistics about jobs and system usage",
                "produces": ["application/json"],
                "tags": ["system"],
                "summary": "Get system statistics",
                "responses": {
                    "200": {"description": "OK", "schema": {"type": "object"}}
                }
            }
        },
        "/ws": {
            "get": {
                "description": "WebSocket endpoint for receiving real-time progress updates for a job",
                "tags": ["websocket"],
                "summary": "WebSocket connection for job progress",
                "parameters": [
                    {"type": "string", "description": "Job ID to subscribe to", "name": "job_id", "in": "query", "required": true},
                    {"type": "string", "description": "User ID for tracking", "name": "user_id", "in": "query"}
                ],
                "responses": {
                    "101": {"description": "Switching Protocols - WebSocket connection established"},
                    "400": {"description": "Missing job_id parameter"}
                }
            }
        }
    },
    "definitions": {
        "Scheme": {
            "type": "object",
            "properties": {
                "model": {"type": "string", "example": "KBM"},
                "code": {"type": "string", "example": "KBM-WF01"},
                "name": {"type": "string", "example": "全流程安全校核"},
                "class_name": {"type": "string", "example": "KBMWF01Pipeline"},
                "resource_type": {"type": "string", "example": "CPU"}
            }
        },
        "Job": {
            "type": "object",
            "properties": {
                "job_id": {"type": "string", "example": "550e8400-e29b-41d4-a716-446655440000"},
                "scheme_code": {"type": "string", "example": "KBM-WF01"},
                "user_id": {"type": "string", "example": "user_001"},
                "status": {"type": "string", "enum": ["PENDING", "RUNNING", "SUCCESS", "FAILED"]},
                "progress": {"type": "integer", "minimum": 0, "maximum": 100},
                "data_ref": {"type": "string"},
                "params": {"type": "string"},
                "result_summary": {"type": "string"},
                "error_log": {"type": "string"},
                "created_at": {"type": "string", "format": "date-time"},
                "finished_at": {"type": "string", "format": "date-time"}
            }
        },
        "SubmitJobRequest": {
            "type": "object",
            "required": ["scheme", "data_id"],
            "properties": {
                "scheme": {"type": "string", "example": "KBM-WF01", "description": "Algorithm scheme code"},
                "data_id": {"type": "string", "example": "sample_001", "description": "Reference to input data"},
                "params": {"type": "object", "description": "Algorithm-specific parameters"},
                "user_id": {"type": "string", "example": "user_001", "description": "User identifier"}
            }
        },
        "ProgressMessage": {
            "type": "object",
            "properties": {
                "task_id": {"type": "string"},
                "percentage": {"type": "integer", "minimum": 0, "maximum": 100},
                "message": {"type": "string"},
                "timestamp": {"type": "integer", "format": "int64"}
            }
        },
        "ErrorResponse": {
            "type": "object",
            "properties": {
                "error": {"type": "string", "example": "Invalid request"},
                "message": {"type": "string", "example": "Detailed error message"},
                "code": {"type": "integer", "example": 400}
            }
        },
        "SuccessResponse": {
            "type": "object",
            "properties": {
                "success": {"type": "boolean", "example": true},
                "message": {"type": "string", "example": "Operation completed"}
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{"http", "https"},
	Title:            "Electric Power Digital Domain Backend API",
	Description:      "Backend service API for algorithm orchestration, job management, and real-time progress tracking. Supports high-concurrency job submission, WebSocket-based progress streaming, and comprehensive job lifecycle management.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
