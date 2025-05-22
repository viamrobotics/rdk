package msgpiler

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
)

//go:generate ragel -Z -G2 -o lex.go lex.rl
//go:generate goyacc -o rosbag.go -v rosbag.output rosbag.y

var compilerLock sync.Mutex

// MessageFormat contains everything we need to read given format
type MessageFormat struct {
	Namespace      []string
	Name           string
	Fullname       string
	Fields         []MessageField
	MessageFormats []MessageFormat
}

// MessageField contains single datafield definition
type MessageField struct {
	VariableType      string
	IsComplexType     bool
	IsBaseType        bool
	IsArray           bool
	ArrayLength       int32
	VariableName      string
	Namespace         []string
	ComplexTypeFormat *MessageFormat
	BinaryRead        func(*bytes.Buffer, []byte, int32, int32) int32
}

type messageFormatBuilder struct {
	rootMessageProcessed bool
	isConstant           bool
	finalMessage         *MessageFormat
	journalMessage       *MessageFormat
	messageField         *MessageField
}

var builder *messageFormatBuilder

// DebugPrint dumps selected content from compiled message
func (message *MessageFormat) DebugPrint() {
	fmt.Printf("Root message - namespace: %v, name: %v, fullname: %v\n", message.Namespace, message.Name, message.Fullname)
	fmt.Printf("Fields:\n")
	for i, f := range message.Fields {
		fmt.Printf("%v. type %v name %v\n", i, f.VariableType, f.VariableName)
	}
	fmt.Println("Embedded formats")
	for _, sm := range message.MessageFormats {
		fmt.Printf("namespace: %v, name: %v, fullname: %v\n", sm.Namespace, sm.Name, sm.Fullname)
		for j, f := range sm.Fields {
			fmt.Printf("%v. type %v name %v\n", j, f.VariableType, f.VariableName)
		}
	}
}

func (m *messageFormatBuilder) processDefinition() {
	if m.isConstant {
		m.isConstant = false
	} else {
		m.journalMessage.Fields = append(m.journalMessage.Fields, *m.messageField)
	}
	m.messageField = new(MessageField)
	return
}

func (m *messageFormatBuilder) processBaseType(baseType string) {
	m.messageField.IsBaseType = true
	m.messageField.VariableType = baseType
}

func (m *messageFormatBuilder) processArray() {
	m.messageField.IsArray = true
}

func (m *messageFormatBuilder) processArrayLength(arrayLength int) {
	m.messageField.ArrayLength = int32(arrayLength)
}

func (m *messageFormatBuilder) processComplexType(complexType string) {
	m.messageField.IsComplexType = true
	m.messageField.VariableType = complexType
}

func (m *messageFormatBuilder) processVariable(variableName string) {
	m.messageField.VariableName = variableName
}

func (m *messageFormatBuilder) processNamespace(namespace string) {
	m.messageField.Namespace = append(m.messageField.Namespace, namespace)
}

func (m *messageFormatBuilder) processConstant() {
	m.isConstant = true
}

func (m *messageFormatBuilder) processMessageDefinition() {

	m.processDefinitionEnd()
	m.journalMessage = new(MessageFormat)
	m.journalMessage.Name = m.messageField.VariableType
	m.journalMessage.Namespace = m.messageField.Namespace
	if len(m.journalMessage.Namespace) == 0 {
		m.journalMessage.Fullname = m.journalMessage.Name
	} else {
		m.journalMessage.Fullname = fmt.Sprintf("%s/%s", strings.Join(m.journalMessage.Namespace, "/"), m.journalMessage.Name)
	}
	m.messageField = new(MessageField)
}

func (m *messageFormatBuilder) processDefinitionEnd() {
	if m.rootMessageProcessed {
		m.finalMessage.MessageFormats = append(m.finalMessage.MessageFormats, *m.journalMessage)

	} else {
		m.finalMessage = m.journalMessage
		m.rootMessageProcessed = true
	}
}

// Compile compiles ros msg format into AST
func Compile(source []byte, rootmessageFullName string) (*MessageFormat, error) {
	// Wrap it all into single-threaded wrapper
	compilerLock.Lock()
	defer compilerLock.Unlock()
	// Create new package-wide object
	builder = new(messageFormatBuilder)
	builder.messageField = new(MessageField)
	builder.journalMessage = new(MessageFormat)
	builder.journalMessage.Fullname = rootmessageFullName
	names := strings.Split(rootmessageFullName, "/")
	if len(names) < 1 {
		return nil, fmt.Errorf("Root message name is invalid")
	}
	builder.journalMessage.Name = names[len(names)-1]
	builder.journalMessage.Namespace = names[:len(names)-1]
	// Call parser, that will use created global object
	lexer := newLexer(source)
	e := yyParse(lexer)
	if e != 0 {
		return nil, fmt.Errorf("Error %v", e)
	}
	// Wrap up in progress journalMessage
	builder.processDefinitionEnd()
	return builder.finalMessage, nil
}
