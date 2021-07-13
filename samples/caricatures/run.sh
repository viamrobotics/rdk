#!/bin/zsh
echo "pretrained_neural_net_path : $1"
echo "haar_cascade_facial_detection_path : $2"
echo "person_name : $3"

python ../python/main.py $1 $2 $3
