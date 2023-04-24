package main

import (
	"os"
	"sort"
	"text/template"
)

const htmlHeader = `
<!DOCTYPE html>
<html>
<head>
	<link rel='stylesheet' type='text/css' href='api.css'>
	<style>
		.div_endpoint {
			background-color: #282828;
			padding: 1em;
			margin: 1em auto;
			color: #ebdbb1;
		}

		code {
			color: #98971a;
		}

		.copy_curl {

			background-color: #665c54;
			color: #d5c5a1;
			border: 1px;
			padding: 0.5em 0.5em;
			
		}

		.copy_curl:hover {
			background-color: #504945;
			color: #bdae93;
		}

		body {
			background-color: #1d2021;
		}
	</style>
	<script>
		function CopyCurlExample(endpoint) {
			var text = document.getElementById("curl-"+endpoint).innerText
			navigator.clipboard.writeText(text);
		}
	</script>
</head>
<body>
`

const htmlFooter = `
</body>
</html>
`

const htmlTemplate = `
<div class="div_endpoint">
	<h class="h_endpoint"> <em>{{.Name}}</em> {{.Method}} <code>{{.Path}}</code></h> <br>
	<p class="p_json">{{.Doc}}</p>
	{{range .Middlewares}}
	<p class="p_json">{{.HTMLRequirement}}</p> {{end}}
	{{if .InputString}}
	<p class="p_json"> Example input (as <code>{{.InputString}}</code>) for {{.Name}}:</p>
<pre>
<code>
{{.Input.Sample 0 true}}
</code>
</pre>
	{{end}}
	{{if .OutputString}}
	<p class="p_json"> Example output (as <code>{{.OutputString}}</code>) for {{.Name}}:</p>
<pre>
<code>
{{.Output.Sample 0 true}}
</code>
</pre>
	{{end}}
	<p class="p_json"> Example with CURL: <code id="curl-{{.Name}}">{{.CurlExample}}</code>
	<button onclick="CopyCurlExample('{{.Name}}')" class="copy_curl">Copy Example</button>
	</p>
</div>
`

func generateHTML(objects ObjectStore, endpoints []Endpoint, path string) {
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


	t1 := template.New("html")
	t1, err := t1.Parse(htmlTemplate)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(path + "/api.html")
	if err != nil {
		panic(err)
	}
	file.WriteString("<!-- " + GeneratedFileWarning + " -->\n")
	file.WriteString("<!-- " + GeneratedByMessage + " -->\n")

	file.WriteString(htmlHeader)

	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Path < endpoints[j].Path
	})

	for _, ep := range endpoints {
		err = t1.Execute(file, ep)
		if err != nil {
			panic(err)
		}
	}
	file.WriteString(htmlFooter)
}
