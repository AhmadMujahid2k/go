// +build ignore

package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
	"time"
	"unicode"
)

const (
	fileSuffix = "-fields.go"
)

var (
	sourceTmpl = template.Must(template.New("source").Parse(source))
)

func main() {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", sourceFilter, 0)
	if err != nil {
		log.Fatal(err)
	}

	for pkgName, pkg := range pkgs {
		t := &templateData{
			filename: pkgName + fileSuffix,
			Year:     time.Now().Year(),
			Package:  pkgName,
		}
		for filename, f := range pkg.Files {
			log.Printf("Processing %v...", filename)
			if err := t.processAST(f); err != nil {
				log.Fatal(err)
			}
		}
		if err := t.dump(); err != nil {
			log.Fatal(err)
		}
	}
	log.Print("Done.")
}

func (t *templateData) processAST(f *ast.File) error {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range st.Fields.List {
				ft, ok := field.Type.(*ast.Ident)
				if len(field.Names) == 0 || field.Tag == nil || !ok {
					continue
				}
				if ft.String() != "string" {
					continue
				}
				fieldName := field.Names[0].String()
				fieldTag := jsonTag(field.Tag.Value)
				t.Getters = append(t.Getters, newGetter(fieldName, fieldTag))
			}
		}
	}
	return nil
}

func (t *templateData) dump() error {
	if len(t.Getters) == 0 {
		log.Printf("No getters for %v; skipping.", t.filename)
		return nil
	}

	var buf bytes.Buffer
	if err := sourceTmpl.Execute(&buf, t); err != nil {
		return err
	}
	clean, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}

	log.Printf("Writing %v...", t.filename)
	return ioutil.WriteFile(t.filename, clean, 0644)
}

func newGetter(fieldName, fieldTag string) *getter {
	return &getter{
		FieldName: fieldName,
		FieldTag:  fieldTag,
	}
}

type templateData struct {
	filename string
	Year     int
	Package  string
	Getters  []*getter
}

type getter struct {
	FieldName string
	FieldTag  string
}

func sourceFilter(fi os.FileInfo) bool {
	return fi.Name() == "ipinfo.go"
}

func jsonTag(s string) string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	m := strings.FieldsFunc(s, f) // `json:"hostname"` => ["json", "hostname"]
	return m[1]
}

const source = `// Code generated by gen-fields; DO NOT EDIT.

package {{.Package}}

import (
	"bytes"
	"net"
	"strings"
)

{{range .Getters}}
// Get{{.FieldName}} returns a specific field "{{.FieldTag}}" value from the
// API for the provided ip. If nil was provided instead of ip, it returns
// details for the caller's own IP.
func Get{{.FieldName}}(ip net.IP) (string, error) {
	return c.Get{{.FieldName}}(ip)
}

// Get{{.FieldName}} returns a specific field "{{.FieldTag}}" value from the
// API for the provided ip. If nil was provided instead of ip, it returns
// details for the caller's own IP.
func (c *Client) Get{{.FieldName}}(ip net.IP) (string, error) {
	s := "{{.FieldTag}}"
	if ip != nil {
		s = ip.String() + "/" + s
	}
	if c.Cache == nil {
		return c.request{{.FieldName}}(s)
	}
	v, err := c.Cache.GetOrRequest(s, func() (interface{}, error) {
		return c.request{{.FieldName}}(s)
	})
	if err != nil {
		return "", err
	}
	return v.(string), err
}

func (c *Client) request{{.FieldName}}(s string) (string, error) {
	req, err := c.NewRequest(s)
	if err != nil {
		return "", err
	}
	v := new(bytes.Buffer)
	_, err = c.Do(req, v)
	if err != nil {
		return "", err
	}
	vs := strings.TrimSpace(v.String())
	if vs == "undefined" {
		vs = ""
	}
	return vs, nil
}
{{end}}
`
