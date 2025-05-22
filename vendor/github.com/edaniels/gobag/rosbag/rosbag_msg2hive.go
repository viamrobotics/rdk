package rosbag

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/edaniels/gobag/msgpiler"
	"go.uber.org/zap"
)

var (
	depth   = 2
	typemap = map[string]string{
		"bool":    "boolean",
		"byte":    "smallint",
		"char":    "smallint",
		"int8":    "tinyint",
		"uint8":   "smallint",
		"int16":   "smallint",
		"uint16":  "int",
		"int32":   "int",
		"uint32":  "bigint",
		"int64":   "bigint",
		"uint64":  "bigint",
		"float32": "float",
		"float64": "double",
		"string":  "string",
	}
)

func writeType(mf msgpiler.MessageField, outbuf *bytes.Buffer) {
	for z := 0; z < depth; z++ {
		outbuf.WriteString("\t")
	}
	outbuf.WriteString("`" + strings.ToLower(mf.VariableName) + "`: ")
	if mf.IsArray {
		outbuf.WriteString("array<")
	}
	switch {
	case mf.IsComplexType:
		outbuf.WriteString("struct<")
		complexType := mf.ComplexTypeFormat
		for j, ctf := range complexType.Fields {
			outbuf.WriteString("\n")
			depth++
			writeType(ctf, outbuf)
			depth--
			if j < len(complexType.Fields)-1 {
				outbuf.WriteString(",")
			}
		}
	case mf.VariableType == "time":
		depth++
		outbuf.WriteString("struct<")
		outbuf.WriteString("\n")
		for z := 0; z < depth; z++ {
			outbuf.WriteString("\t")
		}
		outbuf.WriteString("`secs` : bigint,")
		outbuf.WriteString("\n")
		for z := 0; z < depth; z++ {
			outbuf.WriteString("\t")
		}
		outbuf.WriteString("`nsecs` : bigint")
		outbuf.WriteString(">")
		depth--
	case mf.VariableType == "duration":
		depth++
		outbuf.WriteString("struct<")
		outbuf.WriteString("\n")
		for z := 0; z < depth; z++ {
			outbuf.WriteString("\t")
		}
		outbuf.WriteString("`secs` : bigint,")
		outbuf.WriteString("\n")
		for z := 0; z < depth; z++ {
			outbuf.WriteString("\t")
		}
		outbuf.WriteString("`nsecs` : bigint")
		outbuf.WriteString(">")
		depth--
	default:
		varType, ok := typemap[mf.VariableType]
		if !ok {
			// Error condition!!!
			log.Error("Unknown variable type", zap.String("VariableType", mf.VariableType))
			panic("Unknown variable type")
		}
		outbuf.WriteString(varType)
	}
	if mf.IsComplexType {
		outbuf.WriteString(">")
	}
	if mf.IsArray {
		outbuf.WriteString(">")
	}
}

// DumpTableDefinitions dumps HIVE definitions of topics
func DumpTableDefinitions(path string) error {
	var (
		noSlashTopic string
		err          error
		outbuf       bytes.Buffer
	)

	sqlPrefixTemplateString := `
drop table if exists bag.{{.bt}}{{.tablename}}{{.bt}}
;

create external table if not exists bag.{{.bt}}{{.tablename}}{{.bt}} (
	{{.bt}}meta{{.bt}} struct<{{.bt}}botid{{.bt}}:string, 
		{{.bt}}secs{{.bt}}: int,
		{{.bt}}nsecs{{.bt}}: int
	>,
	{{.bt}}data{{.bt}} struct<
`
	sqlPrefixTemplate := template.New("sqlPrefixTemplate")
	sqlPrefixTemplate, err = sqlPrefixTemplate.Parse(sqlPrefixTemplateString)
	if err != nil {
		log.Error("Error on parsing SQL prefix template", zap.Error(err))
		return err
	}

	sqlSuffixTemplateString := `	>
) 
partitioned by (dt string)
row format serde 'org.apache.hadoop.hive.serde2.lazy.LazySimpleSerDe'
with serdeproperties (
	'serialization.format' = '1' ) location '{{.tablename}}/' tblproperties ('has_encrypted_data'='false')
;

msck repair table bag.{{.tablename}}
;
`
	sqlSuffixTemplate := template.New("sqlSuffixTemplate")
	sqlSuffixTemplate, err = sqlSuffixTemplate.Parse(sqlSuffixTemplateString)
	if err != nil {
		log.Error("Error on parsing SQL prefix template", zap.Error(err))
		return err
	}

	compiledMessageMapLock.Lock()
	defer compiledMessageMapLock.Unlock()
	// for every compiled message topic
	for topicKey, hash := range compiledMessagesTopics {
		topic := strings.Split(topicKey, "|")[0]
		if topic[0] == '/' {
			noSlashTopic = strings.ToLower(strings.Replace(topic[1:], "/", "_", -1))
		} else {
			noSlashTopic = strings.ToLower(strings.Replace(topic, "/", "_", -1))
		}

		msg, ok := compiledMessages[hash]
		if !ok {
			log.Error("Channel MD5 hash with no compiled message format")
		}
		params := map[string]string{
			"tablename": strings.ToLower(strings.Replace(noSlashTopic, "-", "_", -1)),
			"bt":        "`",
		}
		prefix := new(bytes.Buffer)
		err = sqlPrefixTemplate.Execute(prefix, params)
		if err != nil {
			log.Error("Error while executing prefix template", zap.Error(err))
			return err
		}
		outbuf.Write(prefix.Bytes())
		for i, m := range msg.Fields {
			writeType(m, &outbuf)
			if i < len(msg.Fields)-1 {
				outbuf.WriteString(",")
			}
			outbuf.WriteString("\n")
		}
		suffix := new(bytes.Buffer)
		err = sqlSuffixTemplate.Execute(suffix, params)
		if err != nil {
			log.Error("Error while executing suffix template", zap.Error(err))
			return err
		}
		outbuf.Write(suffix.Bytes())
		outpath := filepath.Join(path, noSlashTopic+"-"+hash+".sql")
		err = ioutil.WriteFile(outpath, outbuf.Bytes(), 0644)
		if err != nil {
			log.Error("Error on writing message definition", zap.Error(err))
			return err
		}
		outbuf.Reset()
	}
	return nil
}
