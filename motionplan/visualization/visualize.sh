#!/bin/bash
if python3 -u "visualize.py"; then
    rm temp.json
fi
