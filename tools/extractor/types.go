package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"go/ast"
	mrand "math/rand"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Middleware represents a middleware used in the API.
type Middleware struct {
	Name      string
	TypeParam string
}

// HTMLRequirement generates a requirement text to be used in the HTML
// generator.
func (m *Middleware) HTMLRequirement() string {
	switch m.Name {
	case "WithJSON[...]":
		return "Requires JSON input"
	case "AuthenticatedEndpoint":
		return "Required Header: <code>Authentication: Bearer $TOKEN</code>"
	case "WithSessionID":
		return "Required URL parameter: <code>sessionID</code>"
	case "WithAccountID":
		return "Required URL parameter: <code>accountID</code>"
	case "WithTenantID":
		return "Required URL parameter: <code>tenantID</code>"
	case "WithPhotoID":
		return "Required URL parameter: <code>photoID</code>"
	case "AdminEndpoint":
		return "Requires the authenticated user to be an <strong>Admin</strong>"
	case "TenantManagerEndpoint":
		return "Requires the authenticated user to be an <strong>Manager</strong> of <strong>tenantID</strong> from the URL parameter"
	case "CorsHandler":
		return "Has CORS policy. For details consult the source code."
	default:
		panic("I don't know how to make this into a requirement:" + m.Name)
	}
}

// Endpoint represents an endpoint in the API.
type Endpoint struct {
	// The method for this endpoint, i.e. GET
	Method string
	// The path for this endpoint, i.e. /accounts
	Path string
	// The name of this endpoint, set to the name of the handler
	Name string

	// Input json object description
	Input *Object
	// Output json object description
	Output *Object
	// The middlewares used by this endpoint
	Middlewares []Middleware

	// Doc represents the associated documentation comment text.
	Doc string
}

// InputString returns the name of the Input object or empty string if no input
// is required.
func (e Endpoint) InputString() string {
	if e.Input == nil {
		return ""
	}

	return e.Input.Name
}

// OutputString returns the name of the Output object or empty string if no
// output is required.
func (e Endpoint) OutputString() string {
	if e.Output == nil {
		return ""
	}

	return e.Output.Name
}

// CurlExample generate an example usage of the endpoint using 'curl'.
func (e Endpoint) CurlExample() string {
	b := strings.Builder{}
	b.WriteString("curl -X ")
	b.WriteString(e.Method)
	b.WriteRune('\n')
	if e.Input != nil {
		b.WriteString(" --data '")
		b.WriteString(e.Input.Sample(0, false))
		b.WriteString("'\n")
	}
	b.WriteString(" localhost:2023")
	b.WriteString(e.Path)

	return b.String()
}

// DartMethod converts an method name to a Dart-style method name.
// Also used in TypeScript, so TODO: rename this
func (e Endpoint) DartMethod() string {
	return strings.ToLower(e.Method)
}

func (e Endpoint) CamelCase() string {
	start := e.Name[0:1]
	return strings.ToLower(start) + e.Name[1:]
}

func (e Endpoint) TypeScriptParameters() string {
	first := true
	sb := strings.Builder{}
	r, err := regexp.Compile(`\{[^\{]+\}`)
	if err != nil {
		panic(err)
	}
	subs := r.FindAllString(e.Path, -1)
	for _, v := range subs {
		if first {
			first = false
		} else {
			sb.WriteString(", ")
		}
		v = strings.TrimPrefix(v, "{")
		v = strings.TrimSuffix(v, "}")
		sb.WriteString(v)
		sb.WriteString(": string")
	}

	if e.Input != nil {
		if first {
			first = false
		} else {
			sb.WriteString(", ")
		}
		sb.WriteString("request: ")
		sb.WriteString(e.Input.Name)
	}

	if sb.Len() == 0 {
		return ""
	}
	return sb.String()
}

func (e Endpoint) TypeScriptOutput(enclosed bool) string {
	if e.Output == nil {
		if enclosed {
			return ""
		}
		return "unknown"
	}

	name := e.Output.Name
	if enclosed {
		return "<" + name + ">"
	}
	return name
}

func (e Endpoint) TypeScriptPath() string {
	path := `"` + e.Path + `"`
	path = strings.ReplaceAll(path, "{", "\" + ")
	path = strings.ReplaceAll(path, "}", " + \"")
	path = strings.TrimSuffix(path, ` + ""`)
	fmt.Printf("path: %v\n", path)
	return path
}

// Field represents a field in a JSON request/response
type Field struct {
	// Name of the field
	Name string
	// TypeName is the type of that field
	TypeName string
	// OmitEmpty represents if the type is nullable
	OmitEmpty bool
	// Doc represents the associated documentation comment text.
	Doc string
}

// Sample generates an example JSON value for this field in the form of:
//
//	"field": "sample value
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
	case "bool":
		if mrand.Intn(2) == 1 {
			value = "true"
		} else {
			value = "false"
		}
	default:
		panic("unimplemented" + f.TypeName)
	}

	return Quote(f.Name) + ": " + value
}

// DartName returns the Dart-Styled name of the field
func (f Field) DartName() string {
	var b strings.Builder
	splits := strings.Split(f.Name, "_")
	for i, v := range splits {
		if i == 0 {
			b.WriteString(v)
		} else {
			titleCase := cases.Title(language.English, cases.NoLower).String(v)

			b.WriteString(titleCase)
		}
	}
	return b.String()
}

// DartType returns the type that should be used in Dart for this field.
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
	case "bool":
		return "bool"
	default:
		panic("don't know how to dartify " + f.TypeName)
	}
}

// TypeScriptType returns the type that should be used in TypeScript for this
// field.
func (f Field) TypeScriptType() string {
	typename := ""
	switch f.TypeName {
	case "string", "UUID", "Time", "IP":
		typename = "string"
	case "bool":
		typename = "boolean"
	default:
		panic("don't know how to typescriptify " + f.TypeName)
	}
	return typename
}

func (f Field) TypeScriptDefault() string {
	if f.OmitEmpty {
		return "undefined"
	}
	switch f.TypeName {
	case "string", "UUID", "IP", "Time":
		return "\"\""
	case "bool":
		return "false"
	default:
		panic("don't know what the default in TypeScript should be for " + f.TypeName)
	}
}

// UnDartify returns a Dart snippet that does the conversion from the type to
// the JSON value
func (f Field) UnDartify() string {
	switch f.DartType() {
	case "String":
		return ""
	case "DateTime":
		return ".toIso8601String()"
	case "bool":
		return ""
	default:
		panic("don't know how to undartify " + f.TypeName)
	}
}

// Object represent a JSON object, with fields, arrays, and subobjects
type Object struct {
	Name   string
	Fields []Field

	Arrays  map[string]*Object
	Objects map[string]*Object

	// Doc represents the associated documentation comment.
	Doc string
}

// Sample generates a sample JSON.
func (o *Object) Sample(level int, showOmitEmpty bool) string {
	sb := strings.Builder{}
	sb.WriteString(Indent(level))
	sb.WriteString("{\n")
	for i, f := range o.Fields {
		sb.WriteString(Indent(level + 1))
		sb.WriteString(f.Sample())
		if i != (len(o.Fields) - 1) {
			sb.WriteRune(',')
		}
		if f.OmitEmpty && showOmitEmpty {
			sb.WriteString(" // omit if empty")
		}
		sb.WriteRune('\n')
	}
	for k, a := range o.Arrays {

		sb.WriteString(Indent(level + 1))
		sb.WriteString(Quote(k))
		sb.WriteString(": ")
		sb.WriteString("[\n")
		if a.Name == "UUID" {
			sb.WriteString(Indent(level + 2))
			sb.WriteString(Quote(uuid.NewString()))
		} else {
			s := a.Sample(level+2, showOmitEmpty)
			sb.WriteString(s)
		}
		sb.WriteString("\n")

		sb.WriteString(Indent(level + 1))
		sb.WriteString("]\n")
	}
	sb.WriteString(Indent(level))
	sb.WriteString("}")
	return sb.String()
}

func (o *Object) AsTypeScriptFunctionArgs() string {
	sb := strings.Builder{}
	for i, f := range o.Fields {
		if i != 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteString(": ")
		sb.WriteString(f.TypeScriptType())
	}
	return sb.String()
}

func (o *Object) TypeScriptDoc() string {
	return o.Doc
}

func (o *Object) AsTypeScriptArray() string {
	if o.Name == "UUID" {
		return "string"
	}
	return o.Name
}

// ObjectStore stores all the objects, this type alias is used for convenience
type ObjectStore map[string]*Object

func (os ObjectStore) extractStruct(name string, st *ast.StructType) {
	fields := make([]Field, 0)
	arrays := make(map[string]*Object)
	for _, field := range st.Fields.List {
		if field.Names != nil && field.Tag != nil {
			tag, omitempty := fieldFromTag(field.Tag.Value)
			doc := strings.ReplaceAll(field.Doc.Text(), "\n", " ")

			switch t := field.Type.(type) {
			case *ast.Ident:
				fields = append(fields, Field{Name: tag, TypeName: t.Name, OmitEmpty: omitempty, Doc: doc})
			case *ast.SelectorExpr:
				fields = append(fields, Field{Name: tag, TypeName: t.Sel.Name, OmitEmpty: omitempty, Doc: doc})
			case *ast.ArrayType:
				switch elt := t.Elt.(type) {
				case *ast.Ident:
					arrays[tag] = os[elt.Name]
				case *ast.SelectorExpr:
					arrays[tag] = os[elt.Sel.Name]
				default:
					msg := fmt.Sprintf("don't know %#v", elt)
					panic(msg)
				}
			default:
				panic("unhandled type")
			}
		} else if field.Names == nil {
			switch t := field.Type.(type) {
			case *ast.Ident:
				if t.Name == "Response" {
					fields = append(fields, Field{Name: "error", TypeName: "string", OmitEmpty: true})
				}
			default:
				panic("don't know")
			}
		}
	}

	os[name].Name = name
	os[name].Arrays = arrays
	os[name].Fields = fields

}

// NewMiddleware processes the middleware into a Middleware struct
func NewMiddleware(middleware func(http.Handler) http.Handler) Middleware {
	var mw Middleware
	pc := reflect.ValueOf(middleware).Pointer()
	fpn := simpleFunctionName(pc)
	if strings.HasPrefix(fpn, "WithJSON[...]") {
		reqType := findJSONRequestType(middleware)
		mw.TypeParam = reqType
	}

	mw.Name = fpn
	if len(mw.Name) < 3 {
		panic("Illegal middleware name:" + mw.Name)
	}
	return mw
}
