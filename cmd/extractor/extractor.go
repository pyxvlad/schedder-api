package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gitlab.com/vlad.anghel/schedder-api"
)

type Middleware struct {
	Name      string
	TypeParam string
}

type Endpoint struct {
	Method string
	Path   string
	Name   string

	Input       *Object
	Output      *Object
	Middlewares []Middleware
}

func (e Endpoint) InputString() string {
	if e.Input == nil {
		return ""
	}

	return e.Input.Name
}

func (e Endpoint) DartMethod() string {
	return strings.ToLower(e.Method)
}

type Field struct {
	Name     string
	TypeName string
}

func Quote(s string) string {
	return "\"" + s + "\""

}

func (f Field) Sample() string {
	var value string
	switch f.TypeName {
	case "string":
		switch f.Name {
		case "email":
			value = Quote("somebody@example.com")
		case "phone":
			value = Quote("+40743123123")
		case "error":
			value = Quote("some error occured")
		case "name":
			value = Quote("My Name Is")
		case "token":
			var b [16]byte
			_, err := rand.Read(b[:])
			if err != nil {
				panic(err)
			}
			str := base64.StdEncoding.EncodeToString(b[:])
			value = Quote(str)
		case "password":
			value = Quote("hackmenow")
		case "device":
			value = Quote("Schedder Android App 6.6beta")
		default:
			value = Quote("this-was-a-random-string")
		}
	case "Time":
		value = Quote(time.Now().String())
	case "UUID":
		id, err := uuid.NewUUID()
		if err != nil {
			panic(err) // really bro
		}
		value = Quote(id.String())
	case "IP":
		value = Quote("127.0.0.1")
	default:
		panic("unimplemented" + f.TypeName)
	}

	return Quote(f.Name) + ": " + value
}

func (f Field) DartName() string {
	var b strings.Builder
	splits := strings.Split(f.Name, "_")
	for i, v := range splits {
		if i == 0 {
			b.WriteString(v)
		} else {
			b.WriteString(strings.Title(v))
		}
	}
	return b.String()
}

func (f Field) DartType() string {
	switch f.TypeName {
	case "string":
		return "String"
	case "UUID":
		return "String"
	case "Time":
		return "DateTime"
	case "IP":
		return "String"
	default:
		panic("don't know how to dartify " + f.TypeName)
	}
}

func (f Field) UnDartify() string {
	switch f.DartType() {
	case "String":
		return ""
	case "DateTime":
		return ".toIso8601String()"
	default:
		panic("don't know how to undartify " + f.TypeName)
	}
}

type Object struct {
	Name   string
	Fields []Field

	Arrays  map[string]*Object
	Objects map[string]*Object
}

func Indent(level int) string {
	return strings.Repeat("\t", level)
}

func (o *Object) Sample(level int) string {
	sb := strings.Builder{}
	sb.WriteString(Indent(level))
	sb.WriteString("{\n")
	for i, f := range o.Fields {
		sb.WriteString(Indent(level + 1))
		sb.WriteString(f.Sample())
		if i != (len(o.Fields) - 1) {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}
	for k, a := range o.Arrays {
		sb.WriteString(Indent(level + 1))
		sb.WriteString(Quote(k))
		sb.WriteString(": ")
		sb.WriteString("[\n")

		s := a.Sample(level + 2)
		sb.WriteString(s)
		sb.WriteString("\n")

		sb.WriteString(Indent(level + 1))
		sb.WriteString("]\n")
	}
	sb.WriteString(Indent(level))
	sb.WriteString("}")
	return sb.String()
}

type ObjectStore map[string]*Object

func field_from_tag(tag string) string {
	tag = strings.TrimPrefix(tag, "`json:\"")
	splits := strings.Split(tag, ",")
	tag = strings.TrimSuffix(splits[0], "\"`")

	return tag
}

func simple_fpn(pc uintptr) string {
	f := runtime.FuncForPC(pc)
	_, name, _ := strings.Cut(f.Name(), "schedder-api.")
	name = strings.TrimPrefix(name, "(*API).")
	return name
}

func findJsonRequestType(middleware func(http.Handler) http.Handler) string {
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

func (os ObjectStore) extractStruct(name string, st *ast.StructType) {
	arrays := make(map[string]*Object)
	fields := make([]Field, 0)
	for _, field := range st.Fields.List {
		if field.Names != nil && field.Tag != nil {
			tag := field_from_tag(field.Tag.Value)
			//tag := strings.Split(field.Tag.Value, "\"")[1]

			//fmt.Printf("\t\t%T %s %s\n", field.Type, field.Type, tag)
			//fmt.Printf("\t member %s of type %s json %s\n", field.Names[0], typeName, tag)

			switch t := field.Type.(type) {
			case *ast.Ident:
				fields = append(fields, Field{Name: tag, TypeName: t.Name})
			case *ast.SelectorExpr:
				fields = append(fields, Field{Name: tag, TypeName: t.Sel.Name})
			case *ast.ArrayType:
				elt := t.Elt.(*ast.Ident)
				arrays[tag] = os[elt.Name]
			default:
				//fmt.Printf("unhandled field %T %s", t, t)
				panic("unhandled type")
			}
		} else if field.Names == nil {
			if embedded, ok := field.Type.(*ast.Ident); ok {
				if embedded.Name == "Response" {
					//fmt.Printf("---> %T %s, %s\n", field.Type, field.Type, field.Names)
					fields = append(fields, Field{Name: "error", TypeName: "string"})
				}
			}
		}
	}

	os[name].Fields = fields
	os[name].Arrays = arrays
	os[name].Name = name
}

func NewMiddleware(middleware func(http.Handler) http.Handler) (mw Middleware) {
	pc := reflect.ValueOf(middleware).Pointer()
	fpn := simple_fpn(pc)
	if strings.HasPrefix(fpn, "WithJSON[...]") {
		reqType := findJsonRequestType(middleware)
		mw.TypeParam = reqType
	}

	mw.Name = fpn
	return
}

func main() {

	api := schedder.New(nil)

	b := bytes.Buffer{}

	endpoints := make([]Endpoint, 0)

	handle_handlers := func(_ string, pat string, handlers map[string]http.Handler, mws []Middleware) {
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
				handle_handlers(ident, pat, route.Handlers, middlewares)
			} else {
				sub := route.SubRoutes
				f(ident+"\t", pat, sub, middlewares)
			}
		}
	}

	f("", "", api.GetMux(), make([]Middleware, 0))

	ioutil.WriteFile("/tmp/routes.txt", b.Bytes(), 0777)

	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, ".", nil, parser.Mode(0))
	if err != nil {
		panic(err)
	}

	pkg, ok := packages["schedder"]
	if !ok {
		panic("couldn't find schedder package in workdir")
	}

	//fmt.Printf("pkg: %#v\n", pkg)

	objects := make(ObjectStore)

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

	//fmt.Println(strings.Repeat("=", 80))

	//for name, obj := range objects {
	//	fmt.Printf("struct %s\n", name)
	//	for _, field := range obj.fields {
	//		fmt.Printf("\t%s %s\n", field.name, field.typeName)
	//		//fmt.Printf("\tsample: %s\n", field.Sample())
	//	}
	//	for arr, t := range obj.arrays {
	//		fmt.Printf("\t%s []%s\n", arr, t.name)
	//	}

	//	//fmt.Printf("\tsample: \n %s \n", obj.Sample(1))
	//}

	fmt.Println(strings.Repeat("=", 80))

	for _, ep := range endpoints {
		fmt.Println(ep.Name, ep.Method, ep.Path)

		for _, m := range ep.Middlewares {
			if m.Name == "WithJSON[...]" {
				ep.Input = objects[m.TypeParam]
			}
			fmt.Printf("\tmid: %v\n", m)
		}

		ep.Output = objects[ep.Name+"Response"]

		if ep.Input != nil {
			fmt.Printf("\trequest: %s\n", ep.Input.Name)
			fmt.Printf("\tsample request: \n%s\n", ep.Input.Sample(1))
		}

		if ep.Output != nil {
			fmt.Printf("\tresponse: %s\n", ep.Output.Name)
			fmt.Printf("\tsample response: \n%s\n", ep.Output.Sample(1))
		}
	}

	fmt.Println(strings.Repeat("=", 80))

	gen(objects, endpoints)
}
