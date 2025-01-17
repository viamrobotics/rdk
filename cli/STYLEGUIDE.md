# Code Style

Thank you for contributing to the Viam CLI! Please keep in mind the following best practices
when structuring your code.

## Make Arguments Typeful
Your CLI command's action func should be constructed with `createCommandWithT` and should put
all command line argument values in a typed struct. This ensures type correctness and reduces
the risks associated with having contributors manually parse args in each function they write.

### Examples
<details>
<summary>Good:</summary>

```golang
type fooArgs struct {
    Bar int
    Baz string
}

func fooAction(ctx cli.Context, args foo) error {
    bar := args.Bar
    baz := args.Baz
    ...
}

...

cli.Command{
    ...
    Flags: []cli.Flag{
        &cli.StringFlag{
            Name: Baz,
        },
        &cli.IntFlag{
            Name: Bar,
        },
    },
    Action: createCommandWithT[fooArgs](fooAction),
    ...
}
```
</details>

<details>
<summary>Bad:</summary>

```golang
func fooAction(ctx cli.Context) error {
    bar := ctx.Int("Bar")
    baz := ctx.String("Baz)
    ...
}

...

cli.Command{
    Flags: []cli.Flag{
        &cli.StringFlag{
            Name: BazFlag,
        },
        &cli.IntFlag{
            Name: BarFlag,
        },
    },
    Action: fooAction,
    ...
}
```
</details>

## Avoid Flag Bloat
We have a significant number of flags in the CLI already, many of which (e.g., `organization` or
`location`) are used in multiple functions. Instead of adding duplicate flags, prefer reusing
existing flags. If necessary, rename then to indicate their more generic usage. 

### Example
<details>
<summary>Good:</summary>

```diff
-const yourSpecialFlag = "some-cool-flag"
+const generalSpecialFlag = "some-cool-flag"
```
</details>

<details>
<summary>Bad:</summary>

```golang
const yourSpecialFlag = "some-cool-flag"
...
const mySpecialFlag = "some-cool-flag"
```
</details>

## Hide Help From Commands With Subcommands
When a parent level command with child commands exists (e.g., `viam organizations` has `list`,
`logo`, `api-key`, and others as child commands), it makes little sense to show a `help` command
since the parent level command doesn't do anything on its own. 

### Example
<details>
<summary>Necessary diff:</summary>

```diff
cli.Command{
    ...
    Name: "my-parent-command",
+    HideHelpCommand: true,
    Subcommands: []*cli.Command{
        {
            Name: "my-child-command1",
            ...
        },
        {
            Name: "my-child-command2",
            ...
        },
    },
}
```
</details>

## Create Usage Text With Helper Func, Only Specify Required Args
When creating usage text for a CLI command, use the `createUsageText` convenience method
to generate text. Be sure to provide the fully qualified command (less `viam`) as your first
argument, and only include actually required flags in the `requiredFlags` argument.

Additionally, be sure to use the `formatAcceptedValues` convenience method for defining usage
text on a flag where only a discrete set of values are permitted.

### Example
<details>
<summary>Diff:</summary>

```diff
cli.Command{
    ...
    Name: "my-parent-command",
    Subcommands: []*cli.Command{
        Name: "my-child-command",
        Flags: []cli.Flag{
            &cli.StringFlag{
                Name: requiredFlag,
                Required: true,
+                Usage: formatAcceptedValues("passes some required value", 'foo', 'bar, 'baz')
-                Usage: "passes some required value. must be either 'foo', 'bar', or 'baz'"
            },
            &cli.StringFlag{
                Name: optionalFlag,
            },
        },
+        UsageText: createUsageText("my-parent-command my-child-command", []string{requiredFlag}, true, false),
-        UsageText: createUsageText("my-child-command", []string{requiredFlag, optionalFlag}, false, false),
        ...
    }
}
```
</details>

## Use `DefaultText` Field For Defining Default values
Instead of adding extra text to the description of what a field does, we should use the automated
formatting provided by the `DefaultText` field.

### Examples
<details>
<summary>Good:</summary>

```golang
cli.StringFlag{
    Name: fooFlag,
    Usage: "sets value of Foo",
    DefaultText: "foo",
}
```
</details>

<details>
<summary>Bad:</summary>

```golang
cli.StringFlag{
    Name: fooFlag,
    Usage: "sets value of Foo (defaults to foo)",
}
```
</details>
