# Module {{.ModuleName}} 

Provide a description of the purpose of the module and any relevant information.

## Prerequisites

If your model(s) have dependencies, mention them here. For example: You must have a [board](https://docs.viam.com/components/board/) component configured to use {{.ModelTriple}}.
{{.ModelTriple}} uses the board component to access and use the pins on the board.

## Model {{.ModelTriple}}

Provide a description of the model and any relevant information.

### Configuration

The following attribute template can be used to configure this model:

```json
{
  "attribute_1": <float>,
  "attribute_2": <string>
}
```

#### Attributes

The following attributes are available for this model:

| Name          | Type   | Inclusion | Description                |
|---------------|--------|-----------|----------------------------|
| `attribute_1` | float  | Required  | Description of attribute 1 |
| `attribute_2` | string | Optional  | Description of attribute 2 |

#### Example Configuration

```json
{
  "attribute_1": 1.0,
  "attribute_2": "foo"
}
```

### DoCommand

If your model implements DoCommand, provide an example payload of each command that is supported and the arguments that can be used. If your model does not implement DoCommand, remove this section.

#### Example DoCommand

```json
{
  "command_name": {
    "arg1": "foo",
    "arg2": 1
  }
}
```
