import itertools
import math
import json
from functools import partial
import sys
import matplotlib.pyplot as plt
import numpy as np
from matplotlib.animation import FuncAnimation
from mpl_toolkits.mplot3d.art3d import Poly3DCollection
import matplotlib.widgets
import mpl_toolkits.axes_grid1


class Box():
    def __init__(self, ax, color):
        self.artist = Poly3DCollection([[np.zeros(3)]], facecolors=color, linewidths=1, edgecolors='k', alpha=0.5)
        ax.add_collection3d(self.artist)

    def draw(self, vertices):
        faces = np.array([
            [vertices[0], vertices[1], vertices[3], vertices[2]],
            [vertices[4], vertices[5], vertices[7], vertices[6]],
            [vertices[0], vertices[1], vertices[5], vertices[4]],
            [vertices[3], vertices[2], vertices[6], vertices[7]],            
            [vertices[0], vertices[2], vertices[6], vertices[4]],
            [vertices[1], vertices[3], vertices[7], vertices[5]]])

        # verify each face on box is a rectangle
        for face in faces:
            if np.round(np.linalg.norm(face[0] - face[2]), 6) != np.round(np.linalg.norm(face[1] - face[3]), 6):
                print("Error: invalid face in Box")
                sys.exit(1)     
        
        # draw the box
        self.artist.set_verts(faces)
        self.artist.do_3d_projection()
        return self.artist

    def artists(self):
        return self.faces

class EntityGroup():
    def __init__(self, ax, example_data, color):
        self.entities = [Box(ax, color) for _ in range(len(example_data))]
        self.artists = [entity.artist for entity in self.entities]

    def draw(self, entities):
        if len(self.entities) != len(entities):
            print("Error: mismatched number of entities")
            sys.exit(1)   
        self.artists = [self.entities[i].draw(np.array([[vertex['X'], vertex['Y'], vertex['Z']] for vertex in entity])) 
            for i, entity in enumerate(entities)
        ]
        return self.artists

class Scene():
    def __init__(self, ax, example_data):
        self.model = EntityGroup(ax, example_data["model"], 'r')
        # TODO: account for multiple types of obstacles
        self.obstacles = EntityGroup(ax, example_data["obstacles0"], 'b')
        self.artists = list(itertools.chain.from_iterable([self.obstacles.artists, self.model.artists]))

    def draw(self, data):
        # TODO: add some error checking here
        obstacle_artist = self.obstacles.draw(data["obstacles0"])
        model_artist = self.model.draw(data["model"])
        self.artists = list(itertools.chain.from_iterable([obstacle_artist, model_artist]))
        return self.artists

class Video():
    def __init__(self, path, fps=5, loop_delay=2):
        # load scene from file
        fig = plt.figure()
        ax = fig.add_subplot(111, projection='3d', proj_type='persp')
        with open(path) as file:
            self.frames = json.load(file)
        self.scene = Scene(ax, self.frames[0])

        # set plot defaults
        ax.dist=10
        ax.azim=30
        ax.elev=10
        ax.set_xlim(-1000, 1000)
        ax.set_ylim(-1000, 1000)
        ax.set_zlim(-200, 1800)
        ax.set_box_aspect(np.ptp([ax.get_xlim(), ax.get_ylim(), ax.get_zlim()], axis=1))

        # start video
        ani = FuncAnimation(
            fig, 
            self.play, 
            frames=len(self.frames), 
            interval=1000/fps, 
            repeat=loop_delay is not None, 
            repeat_delay=loop_delay * 1000
        )
        plt.show()

    def play(self, frame):
        self.scene.draw(self.frames[frame])


if __name__ == "__main__":
    # if len(sys.argv) != 2:
    #     print("Error: must provide a single .json file to display as a command line argument")
    #     sys.exit(1)
    # video = Video(ax, sys.argv[1])
    video = Video("visualization/temp.json")

