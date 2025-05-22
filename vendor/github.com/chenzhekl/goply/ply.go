// goply is a simple ply file loader.
package goply

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)
import "bufio"

// Ply is the struct containing ply-file-related data.
type Ply struct {
	schema PlySchema
	data   map[string][]PlyElement
}

// PlyElement represents a single element in ply.
type PlyElement map[string]interface{}

type PlySchema struct {
	elementNames []string
	elements     []PlyElementSchema
}

type PlyElementSchema struct {
	number        int
	propertyNames []string
	propertyTypes []string
}

// New constructs a new Ply instance by parsing data from `io.Reader`.
func New(r io.Reader) *Ply {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	ply := &Ply{
		schema: PlySchema{
			elementNames: []string{},
			elements:     []PlyElementSchema{},
		},
		data: map[string][]PlyElement{},
	}

	processHeader(scanner, ply)

	return ply
}

// Elements return the set of elements of given name.
func (ply *Ply) Elements(elem string) []PlyElement {
	return ply.data[elem]
}

// Property returns the property value of a `PlyElement` with name `prop`.
func (elem *PlyElement) Property(prop string) interface{} {
	return (*elem)[prop]
}

func processHeader(scanner *bufio.Scanner, ply *Ply) {
	// Magic string
	magic, ok := getLine(scanner)
	if magic[0] != "ply" || !ok {
		panic("Invalid ply file")
	}

	// Element
	for line, ok := getLine(scanner); ok; line, ok = getLine(scanner) {
		switch line[0] {
		case "format":
			if line[1] != "ascii" {
				panic("Only ascii format is supported for now")
			}
		case "element":
			ply.schema.elementNames = append(ply.schema.elementNames, line[1])
			numElements, err := strconv.Atoi(line[2])
			if err != nil {
				panic("Invalid number of elements " + line[2] + " for " + line[1])
			}
			ply.schema.elements = append(ply.schema.elements, PlyElementSchema{
				number:        numElements,
				propertyNames: []string{},
				propertyTypes: []string{},
			})
		case "property":
			switch line[1] {
			case "char", "short", "int", "uchar", "ushort", "uint", "float", "double":
				propertyNames := ply.schema.elements[len(ply.schema.elements)-1].propertyNames
				propertyTypes := ply.schema.elements[len(ply.schema.elements)-1].propertyTypes
				ply.schema.elements[len(ply.schema.elements)-1].propertyNames = append(propertyNames, line[2])
				ply.schema.elements[len(ply.schema.elements)-1].propertyTypes = append(propertyTypes, line[1])
			case "list":
				propertyNames := ply.schema.elements[len(ply.schema.elements)-1].propertyNames
				propertyTypes := ply.schema.elements[len(ply.schema.elements)-1].propertyTypes
				ply.schema.elements[len(ply.schema.elements)-1].propertyNames = append(propertyNames, line[4])
				ply.schema.elements[len(ply.schema.elements)-1].propertyTypes = append(propertyTypes, fmt.Sprintf("%s %s %s", line[1], line[2], line[3]))
			default:
				panic("Invalid property type: " + line[1])
			}
		case "end_header":
			processBody(scanner, ply)
		case "comment":
			continue
		default:
			panic("Invalid token: " + line[0])
		}
	}
}

func processBody(scanner *bufio.Scanner, ply *Ply) {
	for typeID := 0; typeID < len(ply.schema.elements); typeID++ {
		ply.data[ply.schema.elementNames[typeID]] = make([]PlyElement, ply.schema.elements[typeID].number)
		for elementID := 0; elementID < ply.schema.elements[typeID].number; elementID++ {
			line, ok := getLine(scanner)
			if !ok {
				panic("Invalid ply file")
			}
			ply.data[ply.schema.elementNames[typeID]][elementID] = PlyElement{}
			cursor := 0
			for propID, propType := range ply.schema.elements[typeID].propertyTypes {
				if !strings.HasPrefix(propType, "list") {
					propValue := parseProperty(propType, line[cursor])
					ply.data[ply.schema.elementNames[typeID]][elementID][ply.schema.elements[typeID].propertyNames[propID]] = propValue
					cursor++
				} else {
					types := strings.Fields(propType)
					tmp := parseProperty(types[1], line[cursor])
					numElems := 0
					if tmp, ok := tmp.(int8); ok {
						numElems = int(tmp)
					}
					if tmp, ok := tmp.(uint8); ok {
						numElems = int(tmp)
					}
					if tmp, ok := tmp.(int16); ok {
						numElems = int(tmp)
					}
					if tmp, ok := tmp.(uint16); ok {
						numElems = int(tmp)
					}
					if tmp, ok := tmp.(int32); ok {
						numElems = int(tmp)
					}
					if tmp, ok := tmp.(uint32); ok {
						numElems = int(tmp)
					}
					if numElems == 0 {
						panic("Invalid number of list elements")
					}
					ply.data[ply.schema.elementNames[typeID]][elementID][ply.schema.elements[typeID].propertyNames[propID]] = []interface{}{}
					cursor++

					for listElem := 0; listElem < numElems; listElem++ {
						tmp := parseProperty(types[2], line[cursor])
						slice, _ := ply.data[ply.schema.elementNames[typeID]][elementID][ply.schema.elements[typeID].propertyNames[propID]].([]interface{})
						ply.data[ply.schema.elementNames[typeID]][elementID][ply.schema.elements[typeID].propertyNames[propID]] = append(slice, tmp)
						cursor++
					}
				}
			}
		}
	}
}

func getLine(scanner *bufio.Scanner) ([]string, bool) {
	ok := scanner.Scan()
	if ok {
		return strings.Fields(strings.TrimSpace(scanner.Text())), true
	} else {
		return nil, false
	}
}

func parseProperty(propType string, value string) interface{} {
	var prop interface{}
	switch propType {
	case "char":
		tmp, err := strconv.ParseInt(value, 10, 8)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = int8(tmp)
	case "uchar":
		tmp, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = uint8(tmp)
	case "short":
		tmp, err := strconv.ParseInt(value, 10, 16)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = int16(tmp)
	case "ushort":
		tmp, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = uint16(tmp)
	case "int":
		tmp, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = int32(tmp)
	case "uint":
		tmp, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = uint32(tmp)
	case "float":
		tmp, err := strconv.ParseFloat(value, 32)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = float32(tmp)
	case "double":
		tmp, err := strconv.ParseFloat(value, 64)
		if err != nil {
			panic("Invalid ply file")
		}
		prop = float64(tmp)
	default:
		panic("Invalid ply file")
	}

	return prop
}
