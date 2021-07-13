from imutils import face_utils
import datetime
import imutils
import time
import dlib
import cv2, math
import numpy as np
from imutils import face_utils, rotate_bound

# colors
BLUE = (255,0,0)
GREEN = (0,255,0)
RED = (0,0,255)
YELL = (0,255,255)

# haar cascade classfier data
haar_faces = cv2.CascadeClassifier('caricatures_python/filters/haarcascade_frontalface_default.xml')

# haar cascade feature classification
def apply_Haar_filter(img, haar_cascade,scaleFact = 1.1, minNeigh = 5, minSizeW = 30):
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    features = haar_cascade.detectMultiScale(
        gray,
        scaleFactor=scaleFact,
        minNeighbors=minNeigh,
        minSize=(minSizeW, minSizeW),
        flags=cv2.CASCADE_SCALE_IMAGE
    )
    return features

# instantiate the model, detector, and predictor that will be used to analyze the image
model = "caricatures_python/filters/shape_predictor_68_face_landmarks.dat"
detector = dlib.get_frontal_face_detector()
predictor = dlib.shape_predictor(model)

# begin capturing video
videoCapture = cv2.VideoCapture(0)
cv2.imshow('Video', np.empty((5,5),dtype=float))

# boolean to track if face has been found yet
face_found = False

while cv2.getWindowProperty('Video', 0) >= 0:

    # get the current frame from live video
    ret, frame = videoCapture.read()

    # detect faces from frame, print in console to confirm
    faces = apply_Haar_filter(frame, haar_faces, 1.1 , 5, 30)
    if len(faces) >= 1:
        print("I see a face.")
        time.sleep(.01)
        ret, frame = videoCapture.read()
        faces = apply_Haar_filter(frame, haar_faces, 1.1 , 5, 30)
        if len(faces) >= 1:
            (x_abs, y_abs, w, h) = faces[0]
            min_x = max(0, x_abs - 15)
            min_y = max(0, y_abs - 15)
            max_x = min(cv2.CAP_PROP_FRAME_WIDTH, w + 15)
            max_y = min(cv2.CAP_PROP_FRAME_HEIGHT, h + 15)
            img = frame[min_x:max_x][min_y:max_y]
            cv2.rectangle(frame, (min_x, min_y), (max_x, max_y), RED, 2)
            print(img)
            # cv2.imwrite("face.jpg", img)
    
    # convert frame's color to grayscale
    gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
    
    # find the edges from the current (grayscale) frame
    edges = cv2.Canny(cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY), 50, 150)

    # transform the grayscale edges frame to BGR
    frame = cv2.cvtColor(edges, cv2.COLOR_GRAY2BGR)
    
    #isolate the face, retrieve coordinates that represent a rectangle around it
    for (x, y, w, h) in faces:
        cv2.rectangle(frame, (x, y), (x+w, y+h), BLUE, 2) #blue
        cv2.rectangle(frame, (x-15, y-15), (x+w+15, y+h+15), RED, 2) #blue
        # cv2.imshow('Video', frame[y:y+h, x:x+w])

    min_bottom_left = min(0, 5)
    cv2.imshow('Video', frame[0:])

    key = cv2.waitKey(1) & 0xFF
    # if the `q` key was pressed, break from the loop
    if key == ord("q") or face_found:
    	break