// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"text/template"
)

const OPEN_TRACING_PARAMS_MARKER = "@openTracingParams"

func main() {
	if err := buildTimerLayer(); err != nil {
		log.Fatal(err)
	}
	if err := buildOpenTracingLayer(); err != nil {
		log.Fatal(err)
	}
}

func buildTimerLayer() error {
	code, err := generateLayer("TimerLayer", "timer_layer.go.tmpl")
	if err != nil {
		return err
	}
	formatedCode, err := format.Source(code)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join("timer_layer.go"), formatedCode, 0644)
}

func buildOpenTracingLayer() error {
	code, err := generateLayer("OpenTracingLayer", "opentracing_layer.go.tmpl")
	if err != nil {
		return err
	}
	formatedCode, err := format.Source(code)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join("opentracing_layer.go"), formatedCode, 0644)
}

type methodParam struct {
	Name string
	Type string
}

type methodData struct {
	Params        []methodParam
	Results       []string
	ParamsToTrace map[string]bool
}

type subStore struct {
	Methods map[string]methodData
}

type storeMetadata struct {
	Name      string
	SubStores map[string]subStore
	Methods   map[string]methodData
}

func extractMethodMetadata(method *ast.Field, src []byte) methodData {
	params := []methodParam{}
	results := []string{}
	paramsToTrace := map[string]bool{}
	ast.Inspect(method.Type, func(expr ast.Node) bool {
		switch e := expr.(type) {
		case *ast.FuncType:
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
						params = append(params, methodParam{Name: paramName.Name, Type: string(src[param.Type.Pos()-1 : param.Type.End()-1])})
					}
				}
			}
			if e.Results != nil {
				for _, result := range e.Results.List {
					results = append(results, string(src[result.Type.Pos()-1:result.Type.End()-1]))
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
		}
		return true
	})
	return methodData{Params: params, Results: results, ParamsToTrace: paramsToTrace}
}

func extractStoreMetadata() (*storeMetadata, error) {
	// Create the AST by parsing src.
	fset := token.NewFileSet() // positions are relative to fset

	file, err := os.Open("store.go")
	if err != nil {
		return nil, fmt.Errorf("Unable to open store/store.go file: %w", err)
	}
	src, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	file.Close()
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, err
	}

	topLevelFunctions := map[string]bool{
		"MarkSystemRanUnitTests":   false,
		"Close":                    false,
		"LockToMaster":             false,
		"UnlockFromMaster":         false,
		"DropAllTables":            false,
		"TotalMasterDbConnections": true,
		"TotalReadDbConnections":   true,
		"SetContext":               true,
		"TotalSearchDbConnections": true,
		"GetCurrentSchemaVersion":  true,
	}

	metadata := storeMetadata{Methods: map[string]methodData{}, SubStores: map[string]subStore{}}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if x.Name.Name == "Store" {
				for _, method := range x.Type.(*ast.InterfaceType).Methods.List {
					methodName := method.Names[0].Name
					if _, ok := topLevelFunctions[methodName]; ok {
						metadata.Methods[methodName] = extractMethodMetadata(method, src)
					}
				}
			} else if strings.HasSuffix(x.Name.Name, "Store") {
				subStoreName := strings.TrimSuffix(x.Name.Name, "Store")
				metadata.SubStores[subStoreName] = subStore{Methods: map[string]methodData{}}
				for _, method := range x.Type.(*ast.InterfaceType).Methods.List {
					methodName := method.Names[0].Name
					metadata.SubStores[subStoreName].Methods[methodName] = extractMethodMetadata(method, src)
				}
			}
		}
		return true
	})

	return &metadata, nil
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
			if len(results) == 0 {
				return ""
			}
			if len(results) == 1 {
				return strings.Join(results, ", ")
			}
			return fmt.Sprintf("(%s)", strings.Join(results, ", "))
		},
		"genResultsVars": func(results []string) string {
			vars := []string{}
			for i := range results {
				vars = append(vars, fmt.Sprintf("resultVar%d", i))
			}
			return strings.Join(vars, ", ")
		},
		"errorToBoolean": func(results []string) string {
			for i, typeName := range results {
				if typeName == "*model.AppError" {
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
			paramsNames := make([]string, 0, len(params))
			for _, param := range params {
				paramsNames = append(paramsNames, param.Name)
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

	t := template.Must(template.New(templateFile).Funcs(myFuncs).ParseFiles("layer_generators/" + templateFile))
	if err = t.Execute(out, metadata); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
