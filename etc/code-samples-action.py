import json

data = None
with open("./examples/apis.json") as f:
    data = json.load(f)
with open("code-samples-warning.md", "w") as f:
    f.write("Warning your change may break code samples. ")
    f.write("If your change modifies any of the following functions please contact @viamrobotics/fleet-management. Thanks!\n")
    f.write("|component|function|\n")
    f.write("|-|-|\n")
    for k, v in data.items():
        func = v["func"]
        f.write(f"|{k}|{func}|\n")
