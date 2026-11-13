package tool

import (
	"context"
	"encoding/json"
)

type Tool struct {
	Name        string
	Description string
	Params      Param
	Handler     func(ctx context.Context, args json.RawMessage) (string, error)
}

type Param struct {
	Type        string           `json:"type"` // "object" / "string" / "number" / "integer" / "boolean" / "array"
	Description string           `json:"description,omitempty"`
	Properties  map[string]Param `json:"properties,omitempty"` // type=object 时
	Required    []string         `json:"required,omitempty"`   // type=object 时
	Items       *Param           `json:"items,omitempty"`      // type=array 时
	Enum        []any            `json:"enum,omitempty"`
	Default     any              `json:"default,omitempty"`
}

func Object(props map[string]Param, required ...string) Param {
	return Param{Type: "object", Properties: props, Required: required}
}

func String(desc string) Param {
	return Param{Type: "string", Description: desc}
}

func Number(desc string) Param {
	return Param{Type: "number", Description: desc}
}

func Integer(desc string) Param {
	return Param{Type: "integer", Description: desc}
}

func Boolean(desc string) Param {
	return Param{Type: "boolean", Description: desc}
}

func ArrayOf(items Param, desc string) Param {
	return Param{Type: "array", Description: desc, Items: &items}
}

func EnumOf(typ, desc string, values ...any) Param {
	return Param{Type: typ, Description: desc, Enum: values}
}
