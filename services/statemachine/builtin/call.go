package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

var (
	ctxType   = reflect.TypeOf((*context.Context)(nil)).Elem()
	errType   = reflect.TypeOf((*error)(nil)).Elem()
	extraType = reflect.TypeOf(map[string]interface{}{})
)

// findResource looks a dependency up by its full name ("rdk:component:arm/foo")
// or by its short name ("foo"), which must be unique among the dependencies.
func findResource(deps resource.Dependencies, name string) (resource.Resource, error) {
	if strings.Contains(name, ":") || strings.Contains(name, "/") {
		rn, err := resource.NewFromString(name)
		if err != nil {
			return nil, err
		}
		return deps.Lookup(rn)
	}
	var found resource.Resource
	for depName, res := range deps {
		if depName.Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("resource name %q is ambiguous; use its full name", name)
		}
		found = res
	}
	if found == nil {
		return nil, resource.DependencyNotFoundError(resource.Name{Name: name})
	}
	return found, nil
}

// invoke calls method on res by name, mapping args onto its parameters:
// a leading context.Context takes ctx, a map[string]interface{} parameter
// takes args["extra"] (or nothing), a struct parameter is decoded from the
// whole args map, and any other parameter takes the next positional value
// when args is a list, or the single remaining map value otherwise.
// Results are returned as a map: a map result is used as is, any other
// single result is placed under "result".
func invoke(ctx context.Context, res resource.Resource, method string, args interface{}) (map[string]interface{}, error) {
	m := reflect.ValueOf(res).MethodByName(method)
	if !m.IsValid() {
		return nil, fmt.Errorf("resource %q has no method %q", res.Name().ShortName(), method)
	}
	mt := m.Type()

	argMap, _ := args.(map[string]interface{})
	argList, isList := toList(args)
	remaining := map[string]interface{}{}
	for k, v := range argMap {
		if k != "extra" {
			remaining[k] = v
		}
	}
	next := 0

	in := make([]reflect.Value, mt.NumIn())
	for i := range in {
		pt := mt.In(i)
		switch {
		case i == 0 && pt == ctxType:
			in[i] = reflect.ValueOf(ctx)
		case pt == extraType:
			extra, _ := argMap["extra"].(map[string]interface{})
			if extra == nil {
				extra = map[string]interface{}{}
			}
			in[i] = reflect.ValueOf(extra)
		case isList:
			if next >= len(argList) {
				return nil, fmt.Errorf("method %q needs %d arguments, got %d", method, mt.NumIn()-1, len(argList))
			}
			v, err := convertArg(argList[next], pt)
			if err != nil {
				return nil, errors.Wrapf(err, "argument %d of %q", next, method)
			}
			in[i] = v
			next++
		case isStructish(pt):
			v, err := convertArg(remaining, pt)
			if err != nil {
				return nil, errors.Wrapf(err, "arguments of %q", method)
			}
			in[i] = v
			remaining = map[string]interface{}{}
		default:
			if len(remaining) != 1 {
				return nil, fmt.Errorf("method %q: cannot map %d named arguments onto a %s parameter; use a list",
					method, len(remaining), pt)
			}
			var key string
			for k := range remaining {
				key = k
			}
			v, err := convertArg(remaining[key], pt)
			if err != nil {
				return nil, errors.Wrapf(err, "argument %q of %q", key, method)
			}
			in[i] = v
			delete(remaining, key)
		}
	}

	out := m.Call(in)
	results := make([]interface{}, 0, len(out))
	for _, o := range out {
		if o.Type() == errType || o.Type().Implements(errType) && o.Kind() == reflect.Interface {
			if !o.IsNil() {
				return nil, o.Interface().(error)
			}
			continue
		}
		results = append(results, o.Interface())
	}
	switch len(results) {
	case 0:
		return map[string]interface{}{}, nil
	case 1:
		if m, ok := toMap(results[0]); ok {
			return m, nil
		}
		return map[string]interface{}{"result": jsonValue(results[0])}, nil
	default:
		vals := make([]interface{}, len(results))
		for i, r := range results {
			vals[i] = jsonValue(r)
		}
		return map[string]interface{}{"results": vals}, nil
	}
}

func isStructish(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

// convertArg converts a JSON-shaped value into a value of type t.
func convertArg(v interface{}, t reflect.Type) (reflect.Value, error) {
	if v == nil {
		return reflect.Zero(t), nil
	}
	rv := reflect.ValueOf(v)
	if rv.Type().AssignableTo(t) {
		return rv, nil
	}
	if rv.Type().ConvertibleTo(t) && (isNumber(rv.Kind()) && isNumber(t.Kind()) || rv.Kind() == t.Kind()) {
		return rv.Convert(t), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return reflect.Value{}, err
	}
	ptr := reflect.New(t)
	if err := json.Unmarshal(data, ptr.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return ptr.Elem(), nil
}

func isNumber(k reflect.Kind) bool {
	return k >= reflect.Int && k <= reflect.Float64
}

// jsonValue round-trips v through JSON so it is made of maps, lists and
// scalars, as a Struct can carry. Values that do not survive are stringified.
func jsonValue(v interface{}) interface{} {
	switch v.(type) {
	case nil, bool, string, float64:
		return v
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return string(data)
	}
	return out
}

// resolveRefs returns a copy of v in which every string "$ctx.a.b" is
// replaced by input["a"]["b"].
func resolveRefs(v interface{}, input map[string]interface{}) interface{} {
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "$ctx.") {
			return lookupPath(input, strings.Split(strings.TrimPrefix(x, "$ctx."), "."))
		}
		return x
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = resolveRefs(val, input)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = resolveRefs(val, input)
		}
		return out
	default:
		return v
	}
}

func lookupPath(m map[string]interface{}, path []string) interface{} {
	var cur interface{} = m
	for _, p := range path {
		cm, ok := toMap(cur)
		if !ok {
			return nil
		}
		cur = cm[p]
	}
	return cur
}
