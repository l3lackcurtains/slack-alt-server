// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

var (
	reserved           = []string{"AcceptLanguage", "AccountMigration", "Cluster", "Compliance", "Context", "DataRetention", "Elasticsearch", "HTTPService", "ImageProxy", "IpAddress", "Ldap", "Log", "MessageExport", "Metrics", "Notification", "NotificationsLog", "Path", "RequestId", "Saml", "Session", "SetIpAddress", "SetRequestId", "SetSession", "SetStore", "SetT", "Srv", "Store", "T", "Timezones", "UserAgent", "SetUserAgent", "SetAcceptLanguage", "SetPath", "SetContext", "SetServer", "GetT"}
	outputFile         string
	inputFile          string
	outputFileTemplate string
)

const (
	OPEN_TRACING_PARAMS_MARKER = "@openTracingParams"
	APP_ERROR_TYPE             = "*model.AppError"
)

func init() {
	flag.StringVar(&inputFile, "in", path.Join("..", "app_iface.go"), "App interface file")
	flag.StringVar(&outputFile, "out", path.Join("..", "opentracing_layer.go"), "Output file")
	flag.StringVar(&outputFileTemplate, "template", "opentracing_layer.go.tmpl", "Output template file")
}

func main() {
	flag.Parse()

	code, err := generateLayer("OpenTracingAppLayer", outputFileTemplate)
	if err != nil {
		log.Fatal(err)
	}
	formattedCode, err := imports.Process(outputFile, code, &imports.Options{Comments: true})
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(outputFile, formattedCode, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

type methodParam struct {
	Name string
	Type string
}

type methodData struct {
	ParamsToTrace map[string]bool
	Params        []methodParam
	Results       []string
}

type storeMetadata struct {
	Name    string
	Methods map[string]methodData
}

func formatNode(src []byte, node ast.Expr) string {
	return string(src[node.Pos()-1 : node.End()-1])
}

func extractMethodMetadata(method *ast.Field, src []byte) methodData {
	params := []methodParam{}
	paramsToTrace := map[string]bool{}
	results := []string{}
	e := method.Type.(*ast.FuncType)
	if method.Doc != nil {
		for _, comment := range method.Doc.List {
			s := comment.Text
			if idx := strings.Index(s, OPEN_TRACING_PARAMS_MARKER); idx != -1 {
				for _, p := range strings.Split(s[idx+len(OPEN_TRACING_PARAMS_MARKER):], ",") {
					paramsToTrace[strings.TrimSpace(p)] = true
				}
			}

		}
	}
	if e.Params != nil {
		for _, param := range e.Params.List {
			for _, paramName := range param.Names {
				paramType := (formatNode(src, param.Type))
				params = append(params, methodParam{Name: paramName.Name, Type: paramType})
			}
		}
	}
	if e.Results != nil {
		for _, result := range e.Results.List {
			results = append(results, formatNode(src, result.Type))
		}
	}
	for paramName := range paramsToTrace {
		found := false
		for _, param := range params {
			if param.Name == paramName {
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("Unable to find a parameter called '%s' (method '%s') that is mentioned in the '%s' comment. Maybe it was renamed?", paramName, method.Names[0].Name, OPEN_TRACING_PARAMS_MARKER)
		}
	}
	return methodData{Params: params, Results: results, ParamsToTrace: paramsToTrace}
}

func extractStoreMetadata() (*storeMetadata, error) {
	// Create the AST by parsing src.
	fset := token.NewFileSet() // positions are relative to fset

	file, err := os.Open(inputFile)
	if err != nil {
		return nil, fmt.Errorf("Unable to open %s file: %w", inputFile, err)
	}
	src, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	f, err := parser.ParseFile(fset, "../app_iface.go", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, err
	}

	metadata := storeMetadata{Methods: map[string]methodData{}}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if x.Name.Name == "AppIface" {
				for _, method := range x.Type.(*ast.InterfaceType).Methods.List {
					methodName := method.Names[0].Name
					found := false
					for _, reservedMethod := range reserved {
						if methodName == reservedMethod {
							found = true
							break
						}
					}
					if found {
						continue
					}
					metadata.Methods[methodName] = extractMethodMetadata(method, src)
				}
			}
		}

		return true
	})
	return &metadata, err
}

func generateLayer(name, templateFile string) ([]byte, error) {
	out := bytes.NewBufferString("")
	metadata, err := extractStoreMetadata()
	if err != nil {
		return nil, err
	}
	metadata.Name = name

	myFuncs := template.FuncMap{
		"joinResults": func(results []string) string {
			return strings.Join(results, ", ")
		},
		"joinResultsForSignature": func(results []string) string {
			switch len(results) {
			case 0:
				return ""
			case 1:
				return strings.Join(results, ", ")
			}
			return fmt.Sprintf("(%s)", strings.Join(results, ", "))
		},
		"genResultsVars": func(results []string) string {
			vars := make([]string, 0, len(results))
			for i := range results {
				vars = append(vars, fmt.Sprintf("resultVar%d", i))
			}
			return strings.Join(vars, ", ")
		},
		"errorToBoolean": func(results []string) string {
			for i, typeName := range results {
				if typeName == APP_ERROR_TYPE {
					return fmt.Sprintf("resultVar%d == nil", i)
				}
			}
			return "true"
		},
		"errorPresent": func(results []string) bool {
			for _, typeName := range results {
				if typeName == "*model.AppError" {
					return true
				}
			}
			return false
		},
		"errorVar": func(results []string) string {
			for i, typeName := range results {
				if typeName == "*model.AppError" {
					return fmt.Sprintf("resultVar%d", i)
				}
			}
			return ""
		},
		"joinParams": func(params []methodParam) string {
			paramsNames := []string{}
			for _, param := range params {
				s := param.Name
				if strings.HasPrefix(param.Type, "...") {
					s += "..."
				}
				paramsNames = append(paramsNames, s)
			}
			return strings.Join(paramsNames, ", ")
		},
		"joinParamsWithType": func(params []methodParam) string {
			paramsWithType := []string{}
			for _, param := range params {
				paramsWithType = append(paramsWithType, fmt.Sprintf("%s %s", param.Name, param.Type))
			}
			return strings.Join(paramsWithType, ", ")
		},
	}

	t := template.Must(template.New("opentracing_layer.go.tmpl").Funcs(myFuncs).ParseFiles(templateFile))
	err = t.Execute(out, metadata)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
