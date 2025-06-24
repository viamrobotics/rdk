import os
import sys
import subprocess
from sys import platform

def default_callback(str):
    print(str)

def run_tool(callback = default_callback):
    try:
        exe_path = os.path.dirname(os.path.abspath(__file__))
        exe_name = "lidar"
        os.chdir(exe_path)
        cmd = []
        cmd.append("." + os.path.sep + exe_name)
        ps = subprocess.Popen(cmd, shell=False, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=1, universal_newlines=True)

        while True:
            line = ps.stdout.readline()
            if line != '':
                callback(line.strip())
            else:
                break

    except Exception as e:
        print(e)

run_tool()
