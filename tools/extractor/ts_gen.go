package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
)

const tsClassTemplate = `
export class {{.Name}} extends ApiResponse {
{{- range .Fields}}
	{{- if ne .Name "error"}}
	{{.Name}}{{- if .OmitEmpty}}?{{- end}}: {{.TypeScriptType}} = {{.TypeScriptDefault}};
	{{- end}}
{{- end}}
{{- range $name, $arr := .Arrays}}
	{{$name}}: {{$arr.AsTypeScriptArray}}[];
{{- end}}
}
`

// Σ is used instead of backticks in the template in order to not interfere
// with Go's raw strings
const tsConnectionService = `
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Observable, catchError, tap, throwError } from 'rxjs';
import { GenerateTokenRequest, GenerateTokenResponse } from './client';

@Injectable({

  providedIn: 'root'

})
export class ConnectionService {
	constructor(private http: HttpClient){}

	readonly _baseUrl = "http://localhost:2023";

	private handleError(error: HttpErrorResponse) {

		if (error.status === 0) {
		  // A client-side or network error occurred. Handle it accordingly.
		  console.error('An error occurred:', error.error);
		} else {
		  // The backend returned an unsuccessful response code.
		  // The response body may contain clues as to what went wrong.
		  console.error(
			ΣBackend returned code ${error.status} body was: Σ, error.error);
		}
		// Return an observable with a user-facing error message.
		return throwError(() => new Error('Something bad happened; please try again later.'));
	}

{{- range .}}
	{{.CamelCase}}({{.TypeScriptParameters}}): Observable<{{.TypeScriptOutput false}}> {
		return this.http.{{.DartMethod}}{{.TypeScriptOutput true}}(this._baseUrl + {{.TypeScriptPath}}{{- if .InputString}}, request{{- end}})
			.pipe(
				catchError(this.handleError)
			);
	}
{{- end}}
}
`

func generateTypeScript(objects ObjectStore, endpoints []Endpoint, path string) {
	used := make(map[string]bool)
	for k, v := range objects {
		used[k] = false
		for _, e := range endpoints {

			if e.Input == v {
				used[k] = true
			} else if e.Output == v {
				used[k] = true
			}
		}

		if used[k] {
			var f func(obj *Object)
			f = func(obj *Object) {
				for nk, nv := range obj.Arrays {
					used[nk] = true
					f(nv)
				}
				for nk, nv := range obj.Objects {
					used[nk] = true
					f(nv)
				}
			}
		}
	}

	delete(used, "API")

	t1 := template.New("ts")
	t1, err := t1.Parse(tsClassTemplate)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(path + "/client.ts")
	if err != nil {
		panic(err)
	}
	file.WriteString("// " + GeneratedFileWarning + "\n")
	file.WriteString("// " + GeneratedByMessage + "\n")

	// add directly the ApiResponse class, the hardcoded way
	file.WriteString("\nclass ApiResponse {\n\terror?: string = null;\n}\n")

	keys := make([]string, 0, len(used))
	for k := range used {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Name < endpoints[j].Name
	})

	for _, e := range endpoints {
		if e.Name == "GetSessionsForAccount" {
			fmt.Printf("e: %v\n", e)
		}
	}

	for _, name := range keys {
		obj := *objects[name]
		if obj.Name == "Response" {
			continue
		}
		obj.Name = strings.TrimPrefix(obj.Name, "Get")
		err = t1.Execute(file, obj)
		if err != nil {
			panic(err)
		}
	}

	t2 := template.New("connection-service")

	// Σ is used instead of backticks in order to not interfere with Go's raw
	// strings
	t2.Parse(strings.ReplaceAll(tsConnectionService, "Σ", "`"))
	err = t2.Execute(file, endpoints)
	if err != nil {
		panic(err)
	}
}
