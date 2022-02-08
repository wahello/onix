// GENERATED BY THE COMMAND ABOVE; DO NOT EDIT
// This file was generated by swaggo/swag

package docs

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/alecthomas/template"
	"github.com/swaggo/swag"
)

var doc = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{.Description}}",
        "title": "{{.Title}}",
        "contact": {
            "name": "gatblau",
            "url": "http://onix.gatblau.org/",
            "email": "onix@gatblau.org"
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
        "/": {
            "get": {
                "description": "Checks that the HTTP server is listening on the required port.\nUse a liveliness probe.\nIt does not guarantee the server is ready to accept calls.",
                "produces": [
                    "text/plain"
                ],
                "tags": [
                    "General"
                ],
                "summary": "Check that the HTTP API is live",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        },
        "/command": {
            "put": {
                "description": "creates  a new command",
                "produces": [
                    "text/plain"
                ],
                "tags": [
                    "Commands"
                ],
                "summary": "Create a new command",
                "parameters": [
                    {
                        "description": "the data for the command to persist",
                        "name": "key",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/types.Command"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "201": {
                        "description": "Created",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        },
        "/key": {
            "put": {
                "description": "uploads a new key used by doorman for cryptographic operations",
                "produces": [
                    "text/plain"
                ],
                "tags": [
                    "Keys"
                ],
                "summary": "Upload a new key",
                "parameters": [
                    {
                        "description": "the data for the key to persist",
                        "name": "key",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/types.Key"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "201": {
                        "description": "Created",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "types.Command": {
            "type": "object",
            "properties": {
                "description": {
                    "description": "the command description",
                    "type": "string",
                    "example": "scan files in specified path"
                },
                "errorRegex": {
                    "description": "a regex used to determine if the command execution has errored",
                    "type": "string",
                    "example": ".*Infected files: [^0].*"
                },
                "name": {
                    "description": "a unique name for the command",
                    "type": "string",
                    "example": "clamscan"
                },
                "stopOnError": {
                    "description": "determines if the process should stop on a command execution error",
                    "type": "boolean",
                    "example": true
                },
                "value": {
                    "description": "the value of the command",
                    "type": "string",
                    "example": "freshclam \u0026\u0026 clamscan -r ${path}"
                }
            }
        },
        "types.Key": {
            "type": "object",
            "properties": {
                "description": {
                    "description": "a description of the intended use of the key",
                    "type": "string"
                },
                "is_private": {
                    "description": "indicates if the key is private, otherwise public",
                    "type": "boolean"
                },
                "name": {
                    "description": "a unique identifier for the digital key",
                    "type": "string"
                },
                "owner": {
                    "description": "the name of the entity owning the key",
                    "type": "string"
                },
                "value": {
                    "description": "the actual content of the key",
                    "type": "string"
                }
            }
        }
    }
}`

type swaggerInfo struct {
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
	Title       string
	Description string
}

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = swaggerInfo{
	Version:     "0.0.4",
	Host:        "",
	BasePath:    "",
	Schemes:     []string{},
	Title:       "Artisan's Doorman",
	Description: "Transfer (pull, verify, scan, resign and push) artefacts between networks",
}

type s struct{}

func (s *s) ReadDoc() string {
	sInfo := SwaggerInfo
	sInfo.Description = strings.Replace(sInfo.Description, "\n", "\\n", -1)

	t, err := template.New("swagger_info").Funcs(template.FuncMap{
		"marshal": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).Parse(doc)
	if err != nil {
		return doc
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, sInfo); err != nil {
		return doc
	}

	return tpl.String()
}

func init() {
	swag.Register(swag.Name, &s{})
}
