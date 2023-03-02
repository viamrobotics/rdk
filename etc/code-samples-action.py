import json

f = open('./examples/apis.json')
data = json.load(f)
functions = data["functions"]
f.close()
f = open("code-samples-warning.md", "w")
f.write("Warning double check if you have edited any of the following functions if so please contact the fleet team")
f.write("|component|function|")
f.write("|-|-|")
for k,v in functions.items():
    f.write(f"|{k}|{v}|")
f.close()