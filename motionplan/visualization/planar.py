import sys
import json
import numpy as np
import matplotlib.pyplot as plt
import matplotlib.patches as patches


class Path():
    def __init__(self, ax, vertices, color):
        ax.plot(vertices[:, 0], vertices[:, 1], color=color)

    def draw(self):
        # self.artist.set_data()
        self.artist.draw()


def main(file_path):
    fig, ax = plt.subplots()
    fig.canvas.manager.set_window_title('Viam Visualizer')
    with open(file_path) as file:
        paths = json.load(file)
    
    for path in paths:
        Path(ax, np.array([np.array([waypoint[0]['Value'], waypoint[1]['Value']]) for waypoint in path]), 'r')
    rect = patches.Rectangle((-4, 2), 8, 8, linewidth=1, edgecolor='b', facecolor='b')
    ax.add_patch(rect)
    plt.show()


if __name__ == "__main__":
    main("/Users/ray/rdk/motionplan/output.test")    