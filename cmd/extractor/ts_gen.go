package main

import (
	"text/template"
	"os"
)

const tsClassTemplate = `
class {{.Name}} {
	{{- range .Fields}}
	{{.Name}}: {{.TypeScriptType}} = "";
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


	for name := range used {
		err = t1.Execute(file, objects[name])
		if err != nil {
			panic(err)
		}
	}
}

