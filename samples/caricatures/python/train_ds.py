import time
import numpy as np

import torch
import torchvision
import torch.nn as nn
import torch.optim as optim
import torch.nn.functional as F
import torchvision.transforms.functional as TF
from torchvision import datasets, models, transforms
from torch.utils.data import Dataset
from torch.utils.data import DataLoader

import cnn
import io_results

def trainDs(trainLoader, validLoader, images, landmarks, numEpochs):

    # detect outliers (improve performance of neural net) & initialize neural net
    torch.autograd.set_detect_anomaly(True)
    network = cnn.Network()  

    # set loss function and optimizer for neural net
    criterion = nn.MSELoss()
    optimizer = optim.Adam(network.parameters(), lr=0.0001)

    # set minimum loss value and number of epochs
    lossMin = np.inf

    # begin counting time to track performance
    startTime = time.time()

    #begin training process
    for epoch in range(1,numEpochs+1):
        
        lossTrain = 0
        lossValid = 0
        runningLoss = 0
        
        network.train()
        for step in range(1,len(trainLoader)+1):
        
            images, landmarks = next(iter(trainLoader))
            
            landmarks = landmarks.view(landmarks.size(0),-1)
            
            predictions = network(images)
            
            # clear all the gradients before calculating them
            optimizer.zero_grad()
            
            # find the loss for the current step
            lossTrainStep = criterion(predictions, landmarks)
            
            # calculate the gradients
            lossTrainStep.backward()
            
            # update the parameters
            optimizer.step()
            
            lossTrain += lossTrainStep.item()
            runningLoss = lossTrain/step
            
            io_results.printOverwrite(step, len(trainLoader), runningLoss, 'train')
            
        network.eval() 
        with torch.no_grad():
            
            for step in range(1,len(validLoader)+1):
                
                images, landmarks = next(iter(validLoader))

                landmarks = landmarks.view(landmarks.size(0),-1)
            
                predictions = network(images)

                # find the loss for the current step
                lossValidStep = criterion(predictions, landmarks)

                lossValid += lossValidStep.item()
                runningLoss = lossValid/step

                io_results.printOverwrite(step, len(validLoader), runningLoss, 'valid')
        
        lossTrain /= len(trainLoader)
        lossValid /= len(validLoader)
        
        print('\n--------------------------------------------------')
        print('Epoch: {}  Train Loss: {:.4f}  Valid Loss: {:.4f}'.format(epoch, lossTrain, lossValid))
        print('--------------------------------------------------')
        
        if lossValid < lossMin:
            lossMin = lossValid
            torch.save(network.state_dict(), '../face_landmarks1.pth') 
            print("\nMinimum Validation Loss of {:.4f} at epoch {}/{}".format(lossMin, epoch, numEpochs))
            print('Model Saved\n')
        
    print('Training Complete')
    print("Total Elapsed Time : {} s".format(time.time()-startTime))
    