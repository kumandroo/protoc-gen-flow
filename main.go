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
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"fmt"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

const flowFileHeader = `
// @flow

// ------------------------------------
// Code generated by protoc-gen-flow
// source: {{.GetName}}
// THIS FILE IS AUTOMATICALLY GENERATED, DO NOT EDIT!
// ------------------------------------
`

// Map qualified name to Message
var messageMap map[string]*Message

// Map qualified name to Enum
var enumMap map[string]*Enum

func init() {
	messageMap = make(map[string]*Message)
	enumMap = make(map[string]*Enum)
}

func main() {
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	var req plugin.CodeGeneratorRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		log.Fatalf("unable to parse protobuf: %v", err)
	}

	code := bytes.NewBuffer(nil)
	code.WriteString(flowFileHeader)

	for _, f := range req.ProtoFile {
		ns := *f.Package + "$"
		for _, enum := range f.EnumType {
			emitEnumType(code, ns, enum)
		}

		for _, msg := range f.MessageType {
			emitMessageType(code, ns, msg)
		}
	}

	const outputFilename = "index.js"

	emitFiles([]*plugin.CodeGeneratorResponse_File{
		{
			Name:    proto.String(outputFilename),
			Content: proto.String(strings.TrimLeft(code.String(), "\n")),
		},
	})
}

func emitEnumType(code io.Writer, namespace string, enum *descriptor.EnumDescriptorProto) error {
	name := namespace + *enum.Name

	e := &Enum{
		Name:   name,
		Values: []string{},
	}

	for _, v := range enum.Value {
		// Skip default value since we represent it with 'undefined' in JS
		if *v.Number == 0 {
			continue
		}

		e.Values = append(e.Values, *v.Name)
	}

	enumTemplate.Execute(code, e)

	enumMap[name] = e

	return nil
}

func emitMessageType(code io.Writer, namespace string, msg *descriptor.DescriptorProto) error {
	name := namespace + *msg.Name

	m := &Message{
		Name:   name,
		Fields: []*Field{},
		IsMap:  msg.GetOptions().GetMapEntry(),
	}

	nestedNS := name+"$"
	for _, enum := range msg.EnumType {
		emitEnumType(code, nestedNS, enum)
	}

	for _, nestedType := range msg.NestedType {
		emitMessageType(code, nestedNS, nestedType)
	}

	for _, field := range msg.Field {
		m.Fields = append(m.Fields, &Field{
			Name: *field.Name,
			Type: getFieldType(name, field),
		})
	}

	// Map types are inlined
	if !m.IsMap {
		messageTemplate.Execute(code, m)
	}

	messageMap[name] = m

	return nil
}

func emitFiles(out []*plugin.CodeGeneratorResponse_File) {
	emitResp(&plugin.CodeGeneratorResponse{File: out})
}

func emitResp(resp *plugin.CodeGeneratorResponse) {
	buf, err := proto.Marshal(resp)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(buf); err != nil {
		log.Fatal(err)
	}
}

func getFieldType(namespace string, field *descriptor.FieldDescriptorProto) string {
	ret := "any" // unknonwn

	switch *field.Type {
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE,
		descriptor.FieldDescriptorProto_TYPE_FLOAT,
		descriptor.FieldDescriptorProto_TYPE_INT32,
		descriptor.FieldDescriptorProto_TYPE_FIXED32,
		descriptor.FieldDescriptorProto_TYPE_UINT32,
		descriptor.FieldDescriptorProto_TYPE_SFIXED32,
		descriptor.FieldDescriptorProto_TYPE_SINT32:
		ret = "number"
	case descriptor.FieldDescriptorProto_TYPE_INT64,
		descriptor.FieldDescriptorProto_TYPE_UINT64,
		descriptor.FieldDescriptorProto_TYPE_FIXED64,
		descriptor.FieldDescriptorProto_TYPE_SFIXED64,
		descriptor.FieldDescriptorProto_TYPE_SINT64:
		// javascript doesn't support 64bit ints
		ret = "string"
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		ret = "boolean"
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		ret = "string"
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		parts := strings.Split(*field.TypeName, ".")
		if len(parts) < 2 {
			ret = "any"
			break
		}
		parts = parts[1:]

		name := strings.Join(parts, "$")

		_, ok := enumMap[name]
		if !ok {
			panic(fmt.Sprintf("Enum '%v' not found in enum map", name))
		}

		ret = name
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		if *field.TypeName == ".google.protobuf.Timestamp" {
			// Special case for handling timestamps
			ret = "string"
		} else {
			parts := strings.Split(*field.TypeName, ".")
			if len(parts) < 2 {
				ret = "any"
				break
			}
			parts = parts[1:]

			name := strings.Join(parts, "$")

			msg, ok := messageMap[name]
			if ok && msg.IsMap {
				keyType := msg.Fields[0].Type
				valueType := msg.Fields[1].Type

				ret = fmt.Sprintf("{ [key: %s]: %s }", keyType, valueType)

				// Maps are represented as an array of map entries. We change the representation to a JS object so
				// we return early to avoid appending array square braces.
				return ret
			}

			ret = name
		}
	case descriptor.FieldDescriptorProto_TYPE_GROUP:
		ret = "any"
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		ret = "any"
	}
	if *field.Label == descriptor.FieldDescriptorProto_LABEL_REPEATED {
		ret += "[]"
	}

	return ret
}
