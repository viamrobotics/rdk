import sys
import json
import numpy as np
import matplotlib.pyplot as plt
import matplotlib.patches as patches


class Path():
    def __init__(self, file_path):
        with open(file_path) as file:
            paths = json.load(file)
        
        for path in paths:
            self.vertices = np.array([np.array([waypoint[0]['Value'], waypoint[1]['Value']]) for waypoint in path])

    def draw(self, ax, color):
        ax.plot(self.vertices[:, 0], self.vertices[:, 1], color=color)


class Tree():
    def __init__(self, file_path):    
        with open(file_path) as file:
            paths = json.load(file)
        for path in paths:
            self.vertices = np.array([np.array([waypoint[0]['Value'], waypoint[1]['Value']]) for waypoint in path])

    def draw(self, ax, color):
        for i in range(int(len(self.vertices) / 2)):
            ax.plot(self.vertices[2*i:2*i+2, 0], self.vertices[2*i:2*i+2, 1], color=color)


if __name__ == "__main__":
    fig, ax = plt.subplots()
    fig.canvas.manager.set_window_title('Viam Visualizer')
    Tree("/Users/ray/rdk/motionplan/tree.test").draw(ax, 'r')  
    Path("/Users/ray/rdk/motionplan/path.test").draw(ax, 'g')      
    rect = patches.Rectangle((-40, 10), 80, 80, linewidth=1, edgecolor='b', facecolor='b')
    ax.add_patch(rect)
    plt.show()
