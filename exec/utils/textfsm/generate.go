package main

import (
	"bytes"
	"go/format"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/sirikothe/gotextfsm"
)

func fileNameWithoutExtension(fileName string) string {
	if pos := strings.LastIndexByte(fileName, '.'); pos != -1 {
		return fileName[:pos]
	}
	return fileName
}

// insert adds a string to a sorted slice of strings while maintaining the sort order.
//
// Parameters:
// - ss: A sorted slice of strings.
// - s: The string to insert.
//
// Returns:
// - []string: A new slice with the string inserted in sorted order.
func insert(ss []string, s string) []string {
	i := sort.SearchStrings(ss, s)
	ss = append(ss, "")
	copy(ss[i+1:], ss[i:])
	ss[i] = s
	return ss
}

// contains checks if a slice contains a specific string.
//
// Parameters:
// - slice: The slice of strings to search.
// - s: The string to look for.
//
// Returns:
// - bool: True if the string is found, false otherwise.
func contains(slice []string, s string) bool {
	for _, str := range slice {
		if str == s {
			return true
		}
	}
	return false
}

// main is the entry point of the program.
// It reads all `.textfsm` files from the `textfsm` directory and generates Go code for each file.
func main() {
	entries, err := os.ReadDir("textfsm")
	if err != nil {
		panic(err)
	}

	for _, de := range entries {
		if !de.IsDir() && strings.HasSuffix(de.Name(), ".textfsm") {
			generate(path.Join("textfsm", de.Name()), "textfsm")
		}
	}
}

// generate processes a `.textfsm` file and generates Go code based on its content.
//
// Parameters:
// - filename: The path to the `.textfsm` file.
// - packageName: The name of the Go package to generate the code in.
func generate(filename string, packageName string) {
	textFsmTemplate, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	rawName := fileNameWithoutExtension(filepath.Base(filename))

	fsm := gotextfsm.TextFSM{}
	err = fsm.ParseString(string(textFsmTemplate))
	if err != nil {
		panic(err)
	}

	var fields []string
	var fieldTypes []string

	// Extract fields and their types from the TextFSM template.
	for field := range fsm.Values {
		// insert in alphabetical sorted order for determinism
		fields = insert(fields, field)
	}

	for _, field := range fields {
		if contains(fsm.Values[field].Options, "List") {
			fieldTypes = append(fieldTypes, "[]string")
		} else {
			fieldTypes = append(fieldTypes, "string")
		}
	}

	// TODO: make PR to add Order field to gotextfsm
	// for _, field := range fsm.Order {
	// 	fields = append(fields, field)
	// }

	// Prepare data for the code generation template.
	templateData := struct {
		TemplateContent string
		TemplateName    string
		PackageName     string
		Fields          []string
		FieldTypes      []string
	}{
		TemplateContent: string(textFsmTemplate),
		TemplateName:    rawName,
		PackageName:     packageName,
		Fields:          fields,
		FieldTypes:      fieldTypes,
	}
	// Create the package directory if it doesn't exist.
	err = os.MkdirAll(packageName, 0755)
	if err != nil {
		panic(err)
	}
	// Parse and execute the code generation template.
	codeGenForOutfile, err := template.New("codeGenForOutfile").Funcs(funcMap).Parse(genTemplate)
	if err != nil {
		panic(err)
	}

	buffer := bytes.Buffer{}
	err = codeGenForOutfile.ExecuteTemplate(&buffer, "codeGenForOutfile", templateData)
	if err != nil {
		panic(err)
	}
	// Format the generated code.
	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		panic(err)
	}
	// Write the formatted code to a file.
	err = os.WriteFile(filepath.Join(packageName, strcase.ToSnake(rawName))+".gen.go", formatted, 0644)
	if err != nil {
		panic(err)
	}
}

var (
	// funcMap defines custom template functions for string manipulation.
	funcMap = template.FuncMap{
		"toCamel": func(s string) string {
			return strcase.ToCamel(s)
		},
		"toSnake": func(s string) string {
			return strcase.ToSnake(s)
		},
	}

	// genTemplate is the Go template used for code generation.
	genTemplate = `// Code generated from textfsm file
package {{ .PackageName }}

import (
	"reflect"

    "github.com/sirikothe/gotextfsm"
)

var template{{ toCamel .TemplateName }} string = ` + "`{{.TemplateContent}}`" + `

type {{ toCamel .TemplateName }}Row struct {
	{{- range $index, $element := .Fields}}
	{{ toCamel $element }} {{ index $.FieldTypes $index }}
	{{- end }}
}

type {{ toCamel .TemplateName }} struct {
	Rows []{{ toCamel .TemplateName }}Row
}

func (p *{{ toCamel .TemplateName }}) IsGoTextFSMStruct() {}

func (p *{{ toCamel .TemplateName }}) Parse(cliOutput string) error {
    fsm := gotextfsm.TextFSM{}
	if err := fsm.ParseString(template{{ toCamel .TemplateName }}); err != nil {
		return err
	}

    parser := gotextfsm.ParserOutput{}
	if err := parser.ParseTextString(string(cliOutput), fsm, true); err != nil {
		return err
	}

	for _, row := range parser.Dict {
		p.Rows = append(p.Rows,
			{{ toCamel .TemplateName }}Row {
			{{- range $index, $element := .Fields }}
				{{ toCamel $element }}: row["{{ $element }}"].({{ index $.FieldTypes $index }}),
			{{- end }}
			},
		)
	}
	return nil
}

func (m *{{ toCamel .TemplateName }}Row) Compare (expected {{ toCamel .TemplateName }}Row) bool {
	return reflect.DeepEqual(*m, expected)
}
{{ range $index,$field := .Fields }}
func (m *{{ toCamel $.TemplateName }}Row) Get{{ toCamel $field }}() {{ index $.FieldTypes $index }} {
	return m.{{ toCamel $field }}
}
{{ end -}}
{{ range $index,$field := .Fields }}
func (m *{{ toCamel $.TemplateName }}) GetAll{{ toCamel $field }}() []{{ index $.FieldTypes $index }} {
	arr := []{{ index $.FieldTypes $index }}{}
	for _, value := range m.Rows {
		arr = append(arr, value.{{ toCamel $field }})
	}
	return arr
}
{{ end -}}
{{ range $index,$field := .Fields }}
func (m *{{ toCamel $.TemplateName }}Row) Verify{{ toCamel $field }}(value {{ index $.FieldTypes $index }}) bool {
	return reflect.DeepEqual(m.{{ toCamel $field }}, value)
}
{{ end -}}
`
)
