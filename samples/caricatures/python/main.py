import time
import cv2
import os
import random
import numpy as np
import matplotlib.pyplot as plt
from PIL import Image
import imutils
import matplotlib.image as mpimg
from collections import OrderedDict
from math import *
import xml.etree.ElementTree as ET

import torch
import torchvision
import torch.nn as nn
import torch.optim as optim
import torch.nn.functional as F
import torchvision.transforms.functional as TF
from torchvision import datasets, models, transforms
from torch.utils.data import Dataset
from torch.utils.data import DataLoader

import train_ds
import test_trained_model
import transforms as transf
import face_landmarks_dataset as fld

import sys

# CONSTANTS
batchSizeTrainDs = 64
batchSizeValidDs = 9
trainNeuralNet = False
testMultipleNotGetSinglePrediction = False
numEpochs = 20

# parameters to be passed into python
filePathToPretrainedFacialLandmarkNeuralNet = None
frontalFaceHaarCascadePath = None
personToDraw = None
if __name__ == "__main__":
    filePathToPretrainedFacialLandmarkNeuralNet = sys.argv[1]
    frontalFaceHaarCascadePath = sys.argv[2]
    personToDraw = sys.argv[3]

# constants
imagePath = "../images/selfie_"+personToDraw+".jpg"

# if local system does not have dataset, download it
if not os.path.exists('../ibug_300W_large_face_landmark_dataset'):
    os.system("cd ..")
    os.system("wget http://dlib.net/files/data/ibug_300W_large_face_landmark_dataset.tar.gz")
    os.system("tar -xvzf 'ibug_300W_large_face_landmark_dataset.tar.gz'")
    os.system("!rm -r 'ibug_300W_large_face_landmark_dataset.tar.gz'")
    os.system("cd python")

# initialize dataset
dataset = fld.FaceLandmarksDataset(transf.Transforms())

# split the dataset into validation and test sets
lenValidSet = int(0.1*len(dataset))
lenTrainSet = len(dataset) - lenValidSet
trainDataset, validDataset  = torch.utils.data.random_split(dataset , [lenTrainSet, lenValidSet])

# shuffle and batch the datasets
trainLoader = torch.utils.data.DataLoader(trainDataset, batch_size=batchSizeTrainDs, shuffle=True, num_workers=0)
validLoader = torch.utils.data.DataLoader(validDataset, batch_size=batchSizeValidDs, shuffle=True, num_workers=0)

# option between training neural net or loading pretrained neural net is dependent on [TRAIN_NEURAL_NET]
if trainNeuralNet:
    # iterate through DataLoader object to create images and landmarks
    images, landmarks = next(iter(trainLoader))

    # create and train a neural net
    train_ds.trainDs(trainLoader, validLoader, images, landmarks, numEpochs=numEpochs)
else: pass

# option between testing multiple images' performance or getting prediction for single image
if testMultipleNotGetSinglePrediction:
    # test the trained neural net
    test_trained_model.testTrainedModelValidDs(validDataset, validLoader, batchSizeValidDs)
else:
    test_trained_model.testTrainedModelSingleImage(personToDraw, filePathToPretrainedFacialLandmarkNeuralNet, frontalFaceHaarCascadePath)
    