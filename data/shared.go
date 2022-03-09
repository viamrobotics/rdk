package data

import (
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
	"os"
)

//TODO: think of a better file name, shared is garbage

// TODO: length prefix when writing
func Write(c chan *any.Any, target *os.File) error {
	for a := range c {
		bytes, err := proto.Marshal(a)
		if err != nil {
			return err
		}
		_, err = target.Write(bytes)
		if err != nil {
			return err
		}
	}
	return nil
}
