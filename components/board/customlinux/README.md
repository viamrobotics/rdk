## [EXPERIMENTAL] Configuring a custom Linux board
This component supports a board running Linux and requires the user to provide a map of gpio pin names to the corresponding gpio chip and line number. The mappings should be provided in a json file in this format:
```json
{
  "*pins": [
    {
        "*name": "string",
        "*ngpio": "int",
        "*line_number": "int",
        "pwm_chip_sysfs_dir": "string",
        "pwm_id": "int"
    }
  ]
}
```

To configure a new board with these mappings, set the `pin_config_filepath` attribute to the filepath to your json configuration file.