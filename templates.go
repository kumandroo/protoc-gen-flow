//   Copyright 2017 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package main

import (
	"text/template"
)

// Message represents a protobuf message type
type Message struct {
	Name   string
	Fields []*Field
	IsMap  bool
}

// Field represents a protobuf message field type
type Field struct {
	Comment string
	Name    string
	Type    string
}

type Method struct {
	Name         string
	RequestType  string
	ResponseType string
}

type Service struct {
	Name    string
	Methods []*Method
}

var messageTemplate = template.Must(template.New("message").Parse(`
export type {{.Name}} = {
	{{- range $field := .Fields}}
	{{$field.Name}}?: {{$field.Type}},
	{{- end}}
};
`))

// Enum represents a protobuf enum type

type Enum struct {
	Name   string
	Values []string
}

var enumTemplate = template.Must(template.New("enum").Parse(`
export type {{.Name}} =
    {{- range $value := .Values}}
	| '{{$value}}'
	{{- end}}
;
`))

var serviceTemplate = template.Must(template.New("service").Parse(`
export interface {{.Name}} {
    {{- range $method := .Methods}}
    {{$method.Name}}(request: {{$method.RequestType}}, metadata?: any, options?: any, callback: (err: ServiceError, response: {{$method.ResponseType}}) => void): { cancel: () => void };
    {{- end}}
};
`))
