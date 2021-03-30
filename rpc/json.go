package rpc

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoiface"
)

var (
	contextT = reflect.TypeOf((*context.Context)(nil)).Elem()
	messageT = reflect.TypeOf((*protoiface.MessageV1)(nil)).Elem()
)

// CallClientMethodJSON calls a method on the given client by deserializing data
// expected to be from a line format of JSON where the format is:
// <MethodName> [JSON]
func CallClientMethodLineJSON(ctx context.Context, client interface{}, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	dataSplit := bytes.SplitN(data, []byte(" "), 2)
	methodName := string(dataSplit[0])
	clientV := reflect.ValueOf(client)
	clientT := clientV.Type()
	method, ok := clientT.MethodByName(methodName)
	if !ok {
		return nil, fmt.Errorf("method %q does not exist", methodName)
	}
	if method.Type.NumIn() != 4 { // (Client, context.Context, pb.<Method>Request, ...grpc.CallOption)
		return nil, fmt.Errorf("method %q does not look unary", methodName)
	}
	if method.Type.In(1) != contextT {
		return nil, fmt.Errorf("expected method %q first param to be context", methodName)
	}
	if !method.Type.In(2).Implements(messageT) {
		return nil, fmt.Errorf("expected method %q second param to be a proto message", methodName)
	}
	message := reflect.New(method.Type.In(2).Elem()).Interface()
	if len(dataSplit) > 1 && len(dataSplit[1]) > 0 {
		if err := JSONPB.Unmarshal(dataSplit[1], message); err != nil {
			return nil, fmt.Errorf("error unmarshaling into message: %w", err)
		}
	}
	// ignore opts
	rets := clientV.MethodByName(methodName).Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(message),
	})
	if errV := rets[1]; errV.IsValid() && !errV.IsZero() {
		gErr := status.Convert(errV.Interface().(error)).Message()
		return nil, fmt.Errorf("error calling method %q: %s", methodName, gErr)
	}
	resp := rets[0].Interface()
	md, err := JSONPB.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("error marshaling response: %w", err)
	}
	return md, nil
}
