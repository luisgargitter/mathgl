// Copyright 2014 The go-gl/mathgl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

// codegen generates go code from templates. Intended to be
// used with go generate; Also makes mgl64 from mgl32.
// See the invocation in mgl32/util.go for details.
// To use it, just run "go generate github.com/go-gl/mathgl/mgl32"
// (or "go generate" in mgl32 directory).

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type Context struct {
	Comment      string
	TemplateName string
}

type MatrixIter struct {
	M     int // row
	N     int // column
	index int
}

var mgl64RewriteRules = []string{
	"mgl32 -> mgl64",
	"float32 -> float64",
	"f32 -> f64",
	"a.Float32 -> a.Float64",
	"math.MaxFloat32 -> math.MaxFloat64",
	"math.SmallestNonzeroFloat32 -> math.SmallestNonzeroFloat64",
}

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: codegen -template file.tmpl -output file.go")
		fmt.Println("Usage: codegen -mgl64 [-dir ../mgl64]")
		flag.PrintDefaults()
	}

	tmplPath := flag.String("template", "file.tmpl", "template path")
	oPath := flag.String("output", "file.go", "output path")
	mgl64 := flag.Bool("mgl64", false, "make mgl64")
	mgl64Path := flag.String("dir", "../mgl64", "path to mgl64 location")

	flag.Parse()
	if flag.NArg() > 0 || flag.NFlag() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if *mgl64 {
		genMgl64(*mgl64Path)
		return
	}

	tmpl := template.New("").Delims("<<", ">>").Funcs(template.FuncMap{
		"typename":    typenameHelper,
		"elementname": elementNameHelper,
		"iter":        iterHelper,
		"matiter":     matrixIterHelper,
		"enum":        enumHelper,
		"sep":         separatorHelper,
		"repeat":      repeatHelper,
		"add":         addHelper,
		"mul":         mulHelper,
	})
	tmpl = template.Must(tmpl.ParseFiles(*tmplPath))
	tmplName := filepath.Base(*tmplPath)

	oFile, err := os.Create(*oPath)
	if err != nil {
		panic(err)
	}

	context := Context{
		Comment:      "This file is generated by codegen.go; DO NOT EDIT",
		TemplateName: tmplName,
	}
	if err = tmpl.ExecuteTemplate(oFile, tmplName, context); err != nil {
		panic(err)
	}
	oFile.Close()

	if err = rungofmt(*oPath, false, nil); err != nil {
		panic(err)
	}
}

func genMgl64(destPath string) {
	HandleFile := func(source string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(destPath, source)

		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode())
		}
		if !strings.HasSuffix(source, ".go") || info.Name() == "codegen.go" {
			return nil
		}
		if !info.Mode().IsRegular() {
			fmt.Println("Ignored, not a regular file:", source)
			return nil
		}

		in, err := ioutil.ReadFile(source)
		if err != nil {
			return err
		}

		out, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC,
			info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()

		comment := fmt.Sprintf(
			"// This file is generated from mgl32/%s; DO NOT EDIT\n\n",
			filepath.ToSlash(source))
		if _, err = out.WriteString(comment); err != nil {
			return err
		}

		r := strings.NewReplacer("//go:generate ", "//#go:generate ") // We don't want go generate directives in mgl64 package.

		if _, err = r.WriteString(out, string(in)); err != nil {
			return err
		}

		return rungofmt(dest, true, mgl64RewriteRules)
	}

	if err := filepath.Walk(".", HandleFile); err != nil {
		panic(err)
	}
}

func rungofmt(path string, fiximports bool, rewriteRules []string) error {
	args := []string{"-w", path}
	output, err := exec.Command("gofmt", args...).CombinedOutput()

	for i := 0; err == nil && i < len(rewriteRules); i++ {
		args = []string{"-w", "-r", rewriteRules[i], path}
		output, err = exec.Command("gofmt", args...).CombinedOutput()
	}

	if fiximports && err == nil {
		args = []string{"-w", path}
		output, err = exec.Command("goimports", args...).CombinedOutput()
	}

	if err != nil {
		fmt.Println("Error executing gofmt", strings.Join(args, " "))
		os.Stdout.Write(output)
	}

	return err
}

func typenameHelper(m, n int) string {
	if m == 1 {
		return fmt.Sprintf("Vec%d", n)
	}
	if n == 1 {
		return fmt.Sprintf("Vec%d", m)
	}
	if m == n {
		return fmt.Sprintf("Mat%d", m)
	}
	return fmt.Sprintf("Mat%dx%d", m, n)
}

func elementNameHelper(m int) string {
	switch m {
	case 0:
		return "X"
	case 1:
		return "Y"
	case 2:
		return "Z"
	case 3:
		return "W"
	default:
		panic("Can't generate element name")
	}
}

func iterHelper(start, end int) []int {
	iter := make([]int, end-start)
	for i := start; i < end; i++ {
		iter[i] = i
	}
	return iter
}

func matrixIterHelper(rows, cols int) []MatrixIter {
	res := make([]MatrixIter, 0, rows*cols)

	for n := 0; n < cols; n++ {
		for m := 0; m < rows; m++ {
			res = append(res, MatrixIter{
				M:     m,
				N:     n,
				index: n*rows + m,
			})
		}
	}

	return res
}

// Template function that returns slice from its arguments. Indended to be used
// in range loops.
func enumHelper(args ...int) []int {
	return args
}

// Template function to insert commas and '+' in range loops.
func separatorHelper(sep string, iterCond int) string {
	if iterCond > 0 {
		return sep
	}
	return ""
}

// Template function to repeat string 'count' times. Inserting 'sep' between
// repetitions. Also changes all occurrences of '%d' to repetition number.
// For example, repeatHelper(3, "col%d", ",") will output "col0, col1, col2"
func repeatHelper(count int, text string, sep string) string {
	var res bytes.Buffer

	for i := 0; i < count; i++ {
		if i > 0 {
			res.WriteString(sep)
		}
		res.WriteString(strings.Replace(text, "%d", fmt.Sprintf("%d", i), -1))
	}

	return res.String()
}

func addHelper(args ...int) int {
	res := 0
	for _, a := range args {
		res += a
	}
	return res
}

func mulHelper(args ...int) int {
	res := 1
	for _, a := range args {
		res *= a
	}
	return res
}

func (i MatrixIter) String() string {
	return fmt.Sprintf("%d", i.index)
}
