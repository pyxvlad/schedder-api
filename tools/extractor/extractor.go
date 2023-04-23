// The extractor is used for extracting data from the API like endpoints
// (with methods and paths). After the extraction is done it outputs the data
// to the console and also generates Dart, TypeScript files for usage and a
// small HTML for documentation purposes.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-chi/chi/v5"
	"gitlab.com/vlad.anghel/schedder-api"
)

// GeneratedFileWarning is a warning for other people to not edit the generated
// files
const GeneratedFileWarning = "GENERATED FILE! DO NOT EDIT, YOUR CHANGES MAY " +
	"BE OVERWRITTEN"

// GeneratedByMessage is a simple message that signals the file was generated
// using this extractor
const GeneratedByMessage = "Generated by schedder-api/tools/extractor"

// Quote the string
func Quote(s string) string {
	return "\"" + s + "\""
}

// Indent creates a string to be used for identation
func Indent(level int) string {
	return strings.Repeat("\t", level)
}

func fieldFromTag(tag string) (name string, omitempty bool) {
	tag = strings.TrimPrefix(tag, "`json:\"")
	tag = strings.TrimSuffix(tag, "\"`")
	splits := strings.Split(tag, ",")

	if len(splits) > 1 {
		return splits[0], splits[1] == "omitempty"
	}
	return splits[0], false
}

func simpleFunctionName(pc uintptr) string {
	f := runtime.FuncForPC(pc)

	name := f.Name()

	lastSlash := strings.LastIndex(f.Name(), "/")
	name = name[lastSlash+1:]

	if strings.HasPrefix(name, "schedder-api") {
		_, name, _ = strings.Cut(name, "schedder-api.")
		name = strings.TrimPrefix(name, "(*API).")
		name = strings.TrimSuffix(name, "-fm")
		return name
	}

	if strings.HasPrefix(name, "cors") {
		name = strings.TrimPrefix(name, "cors.(*Cors).")
		name = strings.TrimSuffix(name, "-fm")

		return "Cors" + name
	}

	return name
}

func findJSONRequestType(middleware func(http.Handler) http.Handler) string {
	req := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	var requestType reflect.Type

	handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		value := r.Context().Value(schedder.CtxJSON)
		requestType = reflect.TypeOf(value)
	}))

	handler.ServeHTTP(w, req)
	name := strings.TrimPrefix(requestType.String(), "*schedder.")
	return name
}

func main() {
	api := schedder.New(nil, nil, nil, "")

	b := bytes.Buffer{}

	endpoints := make([]Endpoint, 0)

	handleHandlers := func(_ string, pat string, handlers map[string]http.Handler, mws []Middleware) {
		for method, handle := range handlers {

			var endpoint http.Handler

			var ep Endpoint
			ep.Middlewares = make([]Middleware, len(mws))
			copy(ep.Middlewares, mws)

			chain, _ := handle.(*chi.ChainHandler)
			if chain != nil {
				for _, mw := range chain.Middlewares {
					newmw := NewMiddleware(mw)
					//fmt.Printf(ident+"mid: %s %s\n", newmw.name, newmw.typeParam)
					ep.Middlewares = append(ep.Middlewares, newmw)
				}
				endpoint = chain.Endpoint
			} else {
				endpoint = handle
			}

			fpn := runtime.FuncForPC(reflect.ValueOf(endpoint).Pointer()).Name()
			//fmt.Printf(ident+"fpn: %v\n", fpn)
			splits := strings.Split(fpn, ".")
			fpn = splits[len(splits)-1]
			fpn = strings.TrimSuffix(fpn, "-fm")
			//fmt.Printf(ident+"%s %s %s\n", method, fpn, pat)
			request := fmt.Sprintf("%s %s %s\n", method, fpn, pat)

			ep.Name = fpn
			ep.Path = pat
			ep.Method = method

			//fmt.Printf("=== %s %#v\n", ep.name, ep.middlewares)

			endpoints = append(endpoints, ep)
			b.WriteString(request)
		}
	}
	var f func(ident string, pattern string, r chi.Routes, mws []Middleware)
	f = func(ident string, pattern string, r chi.Routes, mws []Middleware) {
		middlewares := make([]Middleware, len(mws))
		copy(middlewares, mws)
		for _, mw := range r.Middlewares() {
			newmw := NewMiddleware(mw)
			middlewares = append(middlewares, newmw)
			//fmt.Printf(ident+"mid: %s %s\n", newmw.name, newmw.typeParam)
		}
		for _, route := range r.Routes() {
			pat := strings.TrimSuffix(route.Pattern, "/*")
			pat = pattern + pat
			if route.SubRoutes == nil {
				handleHandlers(ident, pat, route.Handlers, middlewares)
			} else {
				sub := route.SubRoutes
				f(ident+"\t", pat, sub, middlewares)
			}
		}
	}

	f("", "", api.Mux(), make([]Middleware, 0))

	os.WriteFile("/tmp/routes.txt", b.Bytes(), 0777)

	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, ".", nil, parser.Mode(0))
	if err != nil {
		panic(err)
	}

	pkg, ok := packages["schedder"]
	if !ok {
		panic("couldn't find schedder package in workdir")
	}

	objects := make(ObjectStore)
	objects["UUID"] = &Object{Name: "UUID",Fields: nil, Arrays: nil, Objects: nil}

	for _, file := range pkg.Files {
		for _, declaration := range file.Decls {
			decl, ok := declaration.(*ast.GenDecl)
			if ok {
				for _, spec := range decl.Specs {
					typespec, ok := spec.(*ast.TypeSpec)
					if ok {
						_, ok := typespec.Type.(*ast.StructType)
						if ok {
							objects[typespec.Name.Name] = new(Object)
						}
					}
				}
			}
		}
	}

	for _, file := range pkg.Files {
		for _, declaration := range file.Decls {
			decl, ok := declaration.(*ast.GenDecl)
			if ok {
				for _, spec := range decl.Specs {
					typespec, ok := spec.(*ast.TypeSpec)
					if ok {
						structtype, ok := typespec.Type.(*ast.StructType)
						if ok {
							objects.extractStruct(typespec.Name.Name, structtype)
						}
					}
				}
			}
		}
	}

	fmt.Println(strings.Repeat("=", 80))

	for i := range endpoints {
		ep := &endpoints[i]
		fmt.Println(ep.Name, ep.Method, ep.Path)

		for _, m := range ep.Middlewares {
			if m.Name == "WithJSON[...]" {
				ep.Input = objects[m.TypeParam]
			}
			fmt.Printf("\tmid: %v\n", m)
		}
		if ep.Input != nil {
			base := strings.TrimSuffix(ep.Input.Name, "Request")
			ep.Output = objects[base+"Response"]
		} else {
			ep.Output = objects[ep.Name+"Response"]
		}

		if ep.Input != nil {
			fmt.Printf("\trequest: %s\n", ep.Input.Name)
			fmt.Printf("\tsample request: \n%s\n", ep.Input.Sample(1, true))
		}

		if ep.Output != nil {
			fmt.Printf("\tresponse: %s\n", ep.Output.Name)
			fmt.Printf("\tsample response: \n%s\n", ep.Output.Sample(1, true))
		}
	}

	fmt.Println(strings.Repeat("=", 80))

	path := "/tmp"

	err = os.MkdirAll(path, os.ModeDir)
	if err != nil {
		panic(err)
	}

	generateHTML(objects, endpoints, path)
	generateDart(objects, endpoints, path)
	generateTypeScript(objects, endpoints, path)
}
