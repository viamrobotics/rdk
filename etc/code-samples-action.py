import json

f = open('./examples/apis.json')
data = json.load(f)
functions = data["functions"]
f.close()
f = open("code-samples-warning.md", "w")
f.write("Warning changing any of the following functions will break code samples on app if an api for these function changes please contact the fleet team\n")
f.write("|component|function|\n")
f.write("|-|-|\n")
for k, v in functions.items():
    f.write(f"|{k}|{v}|\n")
f.close()
