import math
from functools import partial
import sys
import matplotlib.pyplot as plt
import numpy as np
from matplotlib import animation
from mpl_toolkits.mplot3d.art3d import Poly3DCollection

def visualize_rotation(frame, box):
    angle = math.radians(2) * frame

    points = np.array([[-1, -1, -1],
                       [1, -1, -1],
                       [1, 1, -1],
                       [-1, 1, -1],
                       [-1, -1, 1],
                       [1, -1, 1],
                       [1, 1, 1],
                       [-1, 1, 1]])

    Z = np.zeros((8, 3))
    for i in range(8):
        Z[i, :] = [
            math.cos(angle) * points[i, 0] - math.sin(angle) * points[i, 1],
            math.sin(angle) * points[i, 0] + math.cos(angle) * points[i, 1],
            points[i, 2]
        ]
    Z = 10.0 * Z

    # list of sides' polygons of figure
    vertices = [[Z[0], Z[1], Z[2], Z[3]],
                [Z[4], Z[5], Z[6], Z[7]],
                [Z[0], Z[1], Z[5], Z[4]],
                [Z[2], Z[3], Z[7], Z[6]],
                [Z[1], Z[2], Z[6], Z[5]],
                [Z[4], Z[7], Z[3], Z[0]]]

    # plot sides
    return box.draw(vertices)


class Box():
    def __init__(self, ax):
        self.faces = Poly3DCollection([[np.zeros(3)]], facecolors='white', linewidths=1, edgecolors='r', alpha=0.8)
        ax.add_collection3d(self.faces)

    def draw(self, faces):
        # verify each face on box is a rectangle
        for face in faces:
            if np.round(np.linalg.norm(face[0] - face[2]), 6) != np.round(np.linalg.norm(face[1] - face[3]), 6):
                print("Error: invalid face in Box")
                sys.exit(1)     
        
        # draw the box
        self.faces.set_verts(faces)
        self.faces.do_3d_projection()
        return [self.faces]

def init_func(ax, box):
    ax.set_xlim(-15, 15)
    ax.set_ylim(-15, 15)
    ax.set_zlim(-15, 15)
    ax.set_box_aspect(np.ptp([ax.get_xlim(), ax.get_ylim(), ax.get_zlim()], axis=1))
    return [box.faces]

def animate_rotation():

    fig = plt.figure()
    ax = fig.add_subplot(111, projection='3d', proj_type='persp')

    box = Box(ax)

    anim = animation.FuncAnimation(fig, visualize_rotation, fargs=[box],
                                   init_func=partial(init_func, ax, box),
                                   frames=360, interval=1000 / 30, blit=True)

    plt.show()

animate_rotation()