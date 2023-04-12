package main

import (
	"os"
	"text/template"
)

const structTemplate = `
class {{.Name}} {
	{{- range .Fields}}
	final {{.DartType}} {{.DartName}};
	{{- end}}

	{{- range $name, $arr := .Arrays}}
	final List<{{$arr.Name}}> {{$name}};
	{{- end}}


	{{.Name}}({
		{{- range .Fields}}
		required this.{{.DartName}},
		{{- end}}
		{{- range $name, $arr := .Arrays}}
		required this.{{$name}},
		{{- end}}
	});

	factory {{.Name}}.fromJson(Map<String, dynamic> json) {
		return {{.Name}}(
			{{- range .Fields}}
			{{.DartName}}: json['{{.Name}}'] as String,
			{{- end}}
			{{- range $name, $arr := .Arrays}}
			{{$name}}: (json['{{$name}}'] as List<dynamic>).map((i) => {{$arr.Name}}.fromJson(e as Map<String, dynamic>)).toList()
			{{- end}}
		);
	}

	Map<String, dynamic> toJson() => <String, dynamic>{
		{{- range .Fields}}
		'{{.Name}}': this.{{.DartName}}{{.UnDartify}},
		{{- end}}
		{{- range $name, $arr := .Arrays}}
		'{{$name}}': this.{{$name}}.map((i) => i.toJson()).toList(),
		{{- end}}
	};
}
`

const clientTemplate = `
class ApiClient {
	final http.Client client;

	{{- range .}}
	Future<ceva> {{.Name}}({{.InputString}} arg) {
		var response = await this.client.{{.DartMethod}}(Uri.parse('https://127.0.0.1:2023{{.Path}}'), body: arg.toJson());
		var decodedResponse = jsonDecode(utf8.decode(response.bodyBytes)) as Map;
		// plm
	}
	{{- end}}

}
`

func generateDart(objects ObjectStore, endpoints []Endpoint, path string) {
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

	t1 := template.New("t1")
	t1, err := t1.Parse(structTemplate)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(path + "/classes.dart")
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

	err = file.Close()
	if err != nil {
		panic(err)
	}

	client := template.New("client")
	client, err = client.Parse(clientTemplate)
	if err != nil {
		panic(err)
	}

	clientFile, err := os.Create(path + "/client.dart")
	if err != nil {
		panic(err)
	}

	clientFile.WriteString("// " + GeneratedFileWarning + "\n")
	clientFile.WriteString("// " + GeneratedByMessage + "\n")

	err = client.Execute(clientFile, endpoints)
	if err != nil {
		panic(err)
	}

	err = clientFile.Close()
	if err != nil {
		panic(err)
	}

}
