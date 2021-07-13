from imutils import faceUtils
import datetime
import imutils
import time
import dlib
import cv2, math
import numpy as np

model = "../filters/shape_predictor_68_face_landmarks.dat"
detector = dlib.get_frontal_face_detector()
predictor = dlib.shape_predictor(model)

videoCapture = cv2.VideoCapture(0)
cv2.imshow('Video', np.empty((5,5),dtype=float))

#Filters path
haar_faces = cv2.CascadeClassifier('../filters/haarcascade_frontalface_default.xml')

def applyHaarFilter(img, haar_cascade,scaleFact = 1.1, minNeigh = 5, minSizeW = 30):
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    features = haar_cascade.detectMultiScale(
        gray,
        scaleFactor=scaleFact,
        minNeighbors=minNeigh,
        minSize=(minSizeW, minSizeW),
        flags=cv2.CASCADE_SCALE_IMAGE
    )
    return features

while cv2.getWindowProperty('Video', 0) >= 0:

    #get the current frame from live video
    ret, frame = videoCapture.read()

    # #find the edges from the current frame
    # edges = cv2.Canny(cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY), 50, 150)

    # # Transform the grayscale edges frame to BGR
    # frame = cv2.cvtColor(edges, cv2.COLOR_GRAY2BGR)

    # #display the edges frame
    # cv2.imshow('Video', frame)

    #detect faces from grayscale frame, print in console to confirm
    gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
    # faces = detector(gray, 0)
    faces = applyHaarFilter(frame, haar_faces, 1.3 , 5, 30)
    print("I see", len(faces), "faces.")

    for face in faces:
        #isolate a facial landmark from the face, convert it to numpy format
        shape = predictor(gray, face)
        shape = faceUtils.shape_to_np(shape)

        #find draw the key points
        for (x, y) in shape:
            cv2.circle(frame, (x, y), 1, (255, 0, 0), -1)

        #find key points on the face in form of rectangle
        x,y, w, h = face.left(), face.top(), face.width(), face.height()


    key = cv2.waitKey(1) & 0xFF
    # if the `q` key was pressed, break from the loop
    if key == ord("q"):
    	break