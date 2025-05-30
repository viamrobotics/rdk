package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axw/gocov"
)

// Coverage information
type Coverage struct {
	XMLName         xml.Name  `xml:"coverage"`
	LineRate        float32   `xml:"line-rate,attr"`
	BranchRate      float32   `xml:"branch-rate,attr"`
	LinesCovered    float32   `xml:"lines-covered,attr"`
	LinesValid      int64     `xml:"lines-valid,attr"`
	BranchesCovered int64     `xml:"branches-covered,attr"`
	BranchesValid   int64     `xml:"branches-valid,attr"`
	Complexity      float32   `xml:"complexity,attr"`
	Version         string    `xml:"version,attr"`
	Timestamp       int64     `xml:"timestamp,attr"`
	Packages        []Package `xml:"packages>package"`
	Sources         []string  `xml:"sources>source"`
}

// Package information
type Package struct {
	Name       string  `xml:"name,attr"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Complexity float32 `xml:"complexity,attr"`
	Classes    []Class `xml:"classes>class"`
	LineCount  int64   `xml:"line-count,attr"`
	LineHits   int64   `xml:"line-hits,attr"`
}

// Class information
type Class struct {
	Name       string   `xml:"name,attr"`
	Filename   string   `xml:"filename,attr"`
	LineRate   float32  `xml:"line-rate,attr"`
	BranchRate float32  `xml:"branch-rate,attr"`
	Complexity float32  `xml:"complexity,attr"`
	Methods    []Method `xml:"methods>method"`
	Lines      []Line   `xml:"lines>line"`
	LineCount  int64    `xml:"line-count,attr"`
	LineHits   int64    `xml:"line-hits,attr"`
}

// Method information
type Method struct {
	Name       string  `xml:"name,attr"`
	Signature  string  `xml:"signature,attr"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Complexity float32 `xml:"complexity,attr"`
	Lines      []Line  `xml:"lines>line"`
	LineCount  int64   `xml:"line-count,attr"`
	LineHits   int64   `xml:"line-hits,attr"`
}

// Line information
type Line struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

func main() {
	sourcePathPtr := flag.String(
		"source",
		"",
		"Absolute path to source. Defaults to current working directory.",
	)

	flag.Parse()

	// Parse the commandline arguments.
	var sourcePath string
	var err error
	if *sourcePathPtr != "" {
		sourcePath = *sourcePathPtr
		if !filepath.IsAbs(sourcePath) {
			panic(fmt.Sprintf("Source path is a relative path: %s", sourcePath))
		}
	} else {
		sourcePath, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}

	sources := make([]string, 1)
	sources[0] = sourcePath
	var r struct{ Packages []gocov.Package }
	var totalLines, totalHits int64
	err = json.NewDecoder(os.Stdin).Decode(&r)
	if err != nil {
		panic(err)
	}

	fset := token.NewFileSet()
	tokenFiles := make(map[string]*token.File)

	// convert packages
	packages := make([]Package, len(r.Packages))
	for i, gPackage := range r.Packages {
		// group functions by filename and "class" (type)
		files := make(map[string]map[string]*Class)
		for _, gFunction := range gPackage.Functions {
			// get the releative path by base path.
			fpath, err := filepath.Rel(sourcePath, gFunction.File)
			if err != nil {
				panic(err)
			}
			classes := files[fpath]
			if classes == nil {
				// group functions by "class" (type) in a File
				classes = make(map[string]*Class)
				files[fpath] = classes
			}

			s := strings.Split("-."+gFunction.Name, ".") // className is "-" for package-level functions
			className, methodName := s[len(s)-2], s[len(s)-1]
			class := classes[className]
			if class == nil {
				class = &Class{Name: className, Filename: fpath, Methods: []Method{}, Lines: []Line{}}
				classes[className] = class
			}

			// from github.com/axw/gocov /gocov/annotate.go#printFunctionSource
			// Load the file for line information. Probably overkill, maybe
			// just compute the lines from offsets in here.
			setContent := false
			tokenFile := tokenFiles[gFunction.File]
			if tokenFile == nil {
				info, err := os.Stat(gFunction.File)
				if err != nil {
					panic(err)
				}
				tokenFile = fset.AddFile(gFunction.File, fset.Base(), int(info.Size()))
				setContent = true
			}

			tokenData, err := ioutil.ReadFile(gFunction.File)
			if err != nil {
				panic(err)
			}
			if setContent {
				// This processes the content and records line number info.
				tokenFile.SetLinesForContent(tokenData)
			}

			// convert statements to lines
			lines := make([]Line, len(gFunction.Statements))
			var funcHits int
			for i, s := range gFunction.Statements {
				lineno := tokenFile.Line(tokenFile.Pos(s.Start))
				line := Line{Number: lineno, Hits: s.Reached}
				if int(s.Reached) > 0 {
					funcHits++
				}
				lines[i] = line
				class.Lines = append(class.Lines, line)
			}
			lineRate := float32(funcHits) / float32(len(gFunction.Statements))

			class.Methods = append(class.Methods, Method{Name: methodName, Lines: lines, LineRate: lineRate})
			class.LineCount += int64(len(gFunction.Statements))
			class.LineHits += int64(funcHits)
		}

		// fill package with "classes"
		p := Package{Name: gPackage.Name, Classes: []Class{}}
		for _, classes := range files {
			for _, class := range classes {
				p.LineCount += class.LineCount
				p.LineHits += class.LineHits
				class.LineRate = float32(class.LineHits) / float32(class.LineCount)
				p.Classes = append(p.Classes, *class)
			}
			p.LineRate = float32(p.LineHits) / float32(p.LineCount)
		}
		packages[i] = p
		totalLines += p.LineCount
		totalHits += p.LineHits
	}

	coverage := Coverage{Sources: sources, Packages: packages, Timestamp: time.Now().UnixNano() / int64(time.Millisecond), LinesCovered: float32(totalHits), LinesValid: int64(totalLines), LineRate: float32(totalHits) / float32(totalLines)}

	fmt.Printf(xml.Header)
	fmt.Printf("<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-04.dtd\">\n")

	encoder := xml.NewEncoder(os.Stdout)
	encoder.Indent("", "\t")
	err = encoder.Encode(coverage)
	if err != nil {
		panic(err)
	}

	fmt.Println()
}
