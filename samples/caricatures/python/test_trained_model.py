import torch
import torchvision
import torch.nn as nn
import torch.optim as optim
import torch.nn.functional as F
import torchvision.transforms.functional as TF
from torchvision import datasets, models, transforms
from torch.utils.data import Dataset
from torch.utils.data import DataLoader

import os
import cv2
import time
import numpy as np
from PIL import Image
import matplotlib.pyplot as plt
from matplotlib.widgets import Slider

import cnn
import json
import pathlib

import dlib

# CONSTANTS
numFacialPoints = 68

def writeToJson(filename, xp, yp):
    faceCurvaturePoints = []
    leftBrowPoints = []
    rightBrowPoints = []
    downNosePoints = []
    acrossNostrilsPoints = []
    topLeftEyePoints = []
    bottomLeftEyePoints = []
    topRightEyePoints = []
    bottomRightEyePoints = []
    topOuterLipsPoints = []
    bottomOuterLipsPoints = []
    topInnerLipsPoints = []
    bottomInnerLipsPoints = []
    for i in range(numFacialPoints):
        loc = (i+1)
        x = xp[i]
        y = yp[i]
        if i < 17:
            faceCurvaturePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 22:
            leftBrowPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 27:
            rightBrowPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 31:
            downNosePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 36:
            acrossNostrilsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i == 36 or i == 39:
            topLeftEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
            bottomLeftEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 39:
            topLeftEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 42:
            bottomLeftEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i == 42 or i == 45:
            topRightEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
            bottomRightEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 45:
            topRightEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 48:
            bottomRightEyePoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i == 48 or i == 60 or i == 64 or i == 54:
            topOuterLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
            topInnerLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
            bottomOuterLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
            bottomInnerLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 54:
            topOuterLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 60:
            bottomOuterLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        elif i < 64:
            topInnerLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
        else:
            bottomInnerLipsPoints.append({
                "loc" : loc, "x" : x, "y" : y
            })
    
    data = {}
    data['facial_features'] = []
    data['facial_features'].append({
        "name" : "face_curvature",
        "points" : faceCurvaturePoints
    })
    data['facial_features'].append({
        "name" : "left_brow",
        "points" : leftBrowPoints
    })
    data['facial_features'].append({
        "name" : "right_brow",
        "points" : rightBrowPoints
    })
    data['facial_features'].append({
        "name" : "down_nose",
        "points" : downNosePoints
    })
    data['facial_features'].append({
        "name" : "across_nostrils",
        "points" : acrossNostrilsPoints
    })
    data['facial_features'].append({
        "name" : "top_left_eye",
        "points" : topLeftEyePoints
    })
    data['facial_features'].append({
        "name" : "bottom_left_eye",
        "points" : bottomLeftEyePoints
    })
    data['facial_features'].append({
        "name" : "top_right_eye",
        "points" : topRightEyePoints
    })
    data['facial_features'].append({
        "name" : "bottom_right_eye",
        "points" : bottomRightEyePoints
    })
    data['facial_features'].append({
        "name" : "bottom_outer_lips",
        "points" : bottomOuterLipsPoints
    })
    data['facial_features'].append({
        "name" : "top_outer_lips",
        "points" : topOuterLipsPoints
    })
    data['facial_features'].append({
        "name" : "bottom_inner_lips",
        "points" : bottomInnerLipsPoints
    })
    data['facial_features'].append({
        "name" : "top_inner_lips",
        "points" : topInnerLipsPoints
    })

    with open(('../json/'+filename+'.json'), 'w', encoding='utf-8') as json_file:
        json.dump(data, json_file, indent=4)

def testTrainedModelValidDs(validDataset, validLoader, batchSizeValidDs):

    # begin counting time to track performance
    startTime = time.time()

    # begin predictions process and check against actual results
    with torch.no_grad():

        # initialize neural net from pre-trained model
        network = cnn.Network()
        network.load_state_dict(torch.load(pretrainedModelPath)) 
        network.eval()
        
        # iterate through DataLoader object to create images and landmarks
        images, landmarks = next(iter(validLoader))
        landmarks = (landmarks + 0.5) * 224

        # create predictions based on the loaded network
        predictions = (network(images).cpu() + 0.5) * 224
        predictions = predictions.view(-1,numFacialPoints,2)

        # set up plot to display results
        plt.figure()
        
        # visually print results
        for img_num in range(batchSizeValidDs):
            plt.subplot(batchSizeValidDs/3,3,img_num+1)
            frame = images[img_num].cpu().numpy().transpose(1,2,0)
            plt.imshow(frame, cmap='gray')
            xp = predictions[img_num,:,0]
            yp = predictions[img_num,:,1]
            xl = landmarks[img_num,:,0]
            yl = landmarks[img_num,:,1]
            write_to_json(("facial_landmarks_img_"+str(img_num)), xp, yp)
            plt.scatter(xp, yp, c = 'r', s = 3)
            plt.scatter(xl, yl, c = 'g', s = 3)

        # show plt
        plt.show()

    print('Total number of test images: {}'.format(len(validDataset)))

    endTime = time.time()
    print("Elapsed Time : {}".format(endTime - startTime)) 

# haar cascade feature classification
def apply_Haar_filter(img, haar_cascade,scaleFact = 1.1, minNeigh = 4, minSizeW = 30):
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    features = haar_cascade.detectMultiScale(
        gray,
        scaleFactor=scaleFact,
        minNeighbors=minNeigh,
        minSize=(minSizeW, minSizeW),
        flags=cv2.CASCADE_SCALE_IMAGE
    )
    return features

def testTrainedModelSingleImage(personToDraw, filePathToPretrainedFacialLandmarkNeuralNet, frontalFaceHaarCascadePath):

    # initialize frontal face haar cascade image classification
    frontalFaceHaarCascade = cv2.CascadeClassifier(frontalFaceHaarCascadePath)

    # initialize neural net from pre-trained model
    network = cnn.Network()
    network.load_state_dict(torch.load(filePathToPretrainedFacialLandmarkNeuralNet, map_location=torch.device('cpu')))
    network.eval()

    # begin capturing video
    videoCapture = cv2.VideoCapture(0)
    cv2.imshow('Video', np.empty((5,5),dtype=float))
    face_found = False

    # initialize variables for face and face dimensions
    face = None
    (x, y, w, h) = (0,0,0,0)

    while cv2.getWindowProperty('Video', 0) >= 0:
        # get the current frame from live video
        ret, frame = videoCapture.read()

        # stream live video
        cv2.imshow('Video', frame[0:])

        # detect faces from frame, generate tensor data about face
        faces = apply_Haar_filter(frame, frontalFaceHaarCascade)
        if len(faces) >= 1:
            face = faces[0]
            x, y, w, h = face
            face_found = True
            cv2.imwrite("../images/selfie_"+personToDraw+".jpg", frame)

        key = cv2.waitKey(1) & 0xFF
        # if the `q` key was pressed, break from the loop
        if key == ord("q") or face_found:
            cv2.destroyWindow('Video')
            cv2.VideoCapture().release()
            break

    # read the image that supposedly "has a face in it"
    # and create image objects both for facial landmark
    # prediction and for display purposes
    image = cv2.imread("../images/selfie_"+personToDraw+".jpg")

    grayscaleImage = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    displayImage = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)

    # transform image to format understood by pytorch
    image = grayscaleImage[y:y+h, x:x+h]
    image = TF.resize(Image.fromarray(image), size=(224, 224))
    image = TF.to_tensor(image)
    image = TF.normalize(image, [0.5], [0.5])

    # find landmarks using the neural net
    with torch.no_grad():
        landmarks = network(image.unsqueeze(0)) 
    landmarks = (landmarks.view(numFacialPoints,2).detach().numpy() + 0.5) * np.array([[w, h]]) + np.array([[x, y]])

    # plot the figure containing all facial landmarks and display it
    plt.figure()
    plt.imshow(displayImage)
    
    # create a JSON file containing information about facial landmarks in each picture
    xp, yp = [], []
    for lm in landmarks:
        xp.append(lm[0])
        yp.append(lm[1])

    jsonFileName = "selfie_"+personToDraw
    writeToJson(jsonFileName, xp, yp)
    
    # plot the facial landmark points
    plt.scatter(xp, yp, c = 'c', s = 5)
    plt.show()
