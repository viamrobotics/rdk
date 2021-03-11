package api

import (
	"encoding/json"
	"os"
)

type NewAttributeCreator func() interface{}

type registered struct {
	comptype    ComponentType
	model, attr string
	f           NewAttributeCreator
}

var (
	special = []registered{}
)

func Register(comptype ComponentType, model, attr string, f NewAttributeCreator) {
	special = append(special, registered{comptype, model, attr, f})
}

func findRegisterd(comptype ComponentType, model, attr string) *registered {
	for _, r := range special {
		if r.comptype == comptype && r.model == model && r.attr == attr {
			return &r
		}
	}
	return nil
}

func ReadConfig(fn string) (Config, error) {
	cfg := Config{}

	file, err := os.Open(fn)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return cfg, err
	}

	for idx, c := range cfg.Components {
		for k, v := range c.Attributes {
			s, ok := v.(string)
			if ok {
				cfg.Components[idx].Attributes[k] = os.ExpandEnv(s)
			}

			r := findRegisterd(c.Type, c.Model, k)
			if r != nil {
				n := r.f()
				js, err := json.Marshal(v)
				if err != nil {
					return cfg, err
				}
				err = json.Unmarshal(js, n)
				if err != nil {
					return cfg, err
				}
				cfg.Components[idx].Attributes[k] = n
			}

		}
	}

	return cfg, nil
}
