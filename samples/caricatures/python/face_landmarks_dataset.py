import os
import cv2
import numpy as np
from torch.utils.data import Dataset
import xml.etree.ElementTree as ET

class FaceLandmarksDataset(Dataset):

    def __init__(self, transform=None):

        tree = ET.parse('../ibug_300W_large_face_landmark_dataset/labels_ibug_300W_train.xml')
        root = tree.getroot()

        self.imageFilenames = []
        self.landmarks = []
        self.crops = []
        self.transform = transform
        self.rootDir = '../ibug_300W_large_face_landmark_dataset'
        
        for filename in root[2]:
            self.imageFilenames.append(os.path.join(self.rootDir, filename.attrib['file']))

            self.crops.append(filename[0].attrib)

            landmark = []
            for num in range(68):
                xCoordinate = int(filename[0][num].attrib['x'])
                yCoordinate = int(filename[0][num].attrib['y'])
                landmark.append([xCoordinate, yCoordinate])
            self.landmarks.append(landmark)

        self.landmarks = np.array(self.landmarks).astype('float32')     

        assert len(self.imageFilenames) == len(self.landmarks)

    def __len__(self):
        return len(self.imageFilenames)

    def __getitem__(self, index):
        image = cv2.imread(self.imageFilenames[index], 0)
        landmarks = self.landmarks[index]
        
        if self.transform:
            image, landmarks = self.transform(image, landmarks, self.crops[index])

        landmarks = landmarks - 0.5

        return image, landmarks

