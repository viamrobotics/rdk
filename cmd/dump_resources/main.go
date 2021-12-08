package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"

	// trigger registrations
	_ "go.viam.com/core/robot/impl"
)

var logger = golog.NewDevelopmentLogger("dump_resources")

// Arguments for the command.
type Arguments struct{}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// TODO(erd):
//  - Handle services
func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	compAttrConvs := config.RegisteredComponentAttributeConverters()
	compAttrMapConvs := config.RegisteredComponentAttributeMapConverters()
	svcAttrMapConvs := config.RegisteredServiceAttributeMapConverters()
	_ = svcAttrMapConvs

	dumpResourceInfo := func(res resource.Name, reg interface{}) {
		regV := reflect.ValueOf(reg)
		registrarLoc := regV.FieldByName("RegistrarLoc").Interface()
		fmt.Fprintf(os.Stdout, "\n\nType: %s", res.ResourceType)
		fmt.Fprintf(os.Stdout, "\nSubtype: %s", res.ResourceSubtype)
		if res.Name != "" {
			fmt.Fprintf(os.Stdout, "\nModel: %s", res.Name)
		}

		fmt.Print("\nAttributes:")
		for _, conv := range compAttrConvs {
			if !(conv.Model == res.Name && conv.CompType == config.ComponentType(res.ResourceSubtype)) {
				continue
			}
			fmt.Fprintf(os.Stdout, "\n\tConverted Attribute: %s", conv.Attr)
			break
		}
		var mapConv interface{}
		switch res.ResourceType {
		case resource.ResourceTypeComponent:
			for _, conv := range compAttrMapConvs {
				if !(conv.Model == res.Name && conv.CompType == config.ComponentType(res.ResourceSubtype)) {
					continue
				}
				mapConv = conv.RetType
				break
			}
		case resource.ResourceTypeService:
			for _, conv := range svcAttrMapConvs {
				if conv.SvcType != config.ServiceType(res.ResourceSubtype) {
					continue
				}
				mapConv = conv.RetType
				break
			}
		default:
			panic(fmt.Errorf("unknown resource type %q", res.ResourceType))
		}
		if mapConv == nil {
			fmt.Fprintf(os.Stdout, "\n\tAttributes handled manually; follow Attributes usage at %s", registrarLoc)
		} else {
			var printTypeInfo func(t reflect.Type, indent int)
			printTypeInfo = func(t reflect.Type, indent int) {
				indentStr := strings.Repeat("\t", indent)
				switch t.Kind() {
				case reflect.Ptr:
					fmt.Fprintf(os.Stdout, "(optional) ")
					printTypeInfo(t.Elem(), indent)
				case reflect.Struct:
					for i := 0; i < t.NumField(); i++ {
						f := t.Field(i)
						fmt.Fprintf(os.Stdout, "\n%s%s: ", indentStr, f.Name)
						printTypeInfo(f.Type, indent+1)
					}
				case reflect.Slice:
					fmt.Fprintf(os.Stdout, "[")
					printTypeInfo(t.Elem(), indent+1)
					fmt.Fprintf(os.Stdout, "\n%s]", indentStr)
				case reflect.Map:
					fmt.Fprintf(os.Stdout, "map[%s]", t.Key().String())
					printTypeInfo(t.Elem(), indent+1)
				default:
					fmt.Fprint(os.Stdout, t.String())
				}
			}
			mapConvV := reflect.ValueOf(mapConv)
			if mapConvV.Kind() == reflect.Ptr {
				mapConvV = mapConvV.Elem()
			}
			printTypeInfo(mapConvV.Type(), 1)
		}
	}
	var dumpResourcesInfo func(resources interface{}, resType resource.TypeName, subType resource.SubtypeName)
	dumpResourcesInfo = func(resources interface{}, resType resource.TypeName, subType resource.SubtypeName) {
		resourcesV := reflect.ValueOf(resources)
		for _, key := range resourcesV.MapKeys() {
			res := resourcesV.MapIndex(key)
			if res.Kind() == reflect.Map {
				// sensors, probably, or some deeper hierarchy
				dumpResourcesInfo(res.Interface(), resType, resource.SubtypeName(fmt.Sprintf("%s/%s", subType, key.String())))
				continue
			}
			resName := resource.NewName(resource.ResourceNamespaceCore, resType, subType, key.String())
			dumpResourceInfo(resName, res.Interface())
		}
	}
	dumpComponentsInfo := func(components interface{}, subType resource.SubtypeName) {
		dumpResourcesInfo(components, resource.ResourceTypeComponent, subType)
	}

	components := registry.RegisteredComponents()
	for qModel, reg := range components {
		name, err := resource.NewFromString(qModel)
		if err != nil {
			return err
		}
		if name.Namespace != resource.ResourceNamespaceCore {
			continue
		}

		// The registered components have a key string similar to an FQRN. If that changes
		// this will break.
		dumpResourceInfo(name, reg)
	}

	dumpComponentsInfo(registry.RegisteredBases(), resource.SubtypeName(config.ComponentTypeBase))
	dumpComponentsInfo(registry.RegisteredLidars(), resource.SubtypeName(config.ComponentTypeLidar))
	dumpComponentsInfo(registry.RegisteredSensors(), resource.SubtypeName(config.ComponentTypeSensor))
	dumpComponentsInfo(registry.RegisteredBoards(), resource.SubtypeName(config.ComponentTypeBoard))
	dumpComponentsInfo(registry.RegisteredInputControllers(), resource.SubtypeName(config.ComponentTypeInputController))

	for svcType, reg := range registry.RegisteredServices() {
		resName := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeService,
			resource.SubtypeName(svcType),
			"",
		)
		dumpResourceInfo(resName, reg)
	}

	fmt.Println()
	return nil
}
