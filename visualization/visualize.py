import sys
import json
import itertools
import numpy as np
import mpl_toolkits.axes_grid1
import matplotlib.pyplot as plt
import matplotlib.widgets as widgets
from matplotlib.animation import FuncAnimation
from mpl_toolkits.mplot3d.axes3d import Axes3D
from mpl_toolkits.mplot3d.art3d import Poly3DCollection


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
        # TODO: account for multiple types of obstacles
        self.obstacles = EntityGroup(ax, example_data["obstacles0"], 'b')
        self.model = EntityGroup(ax, example_data["model"], 'r')
        self.artists = list(itertools.chain.from_iterable([self.obstacles.artists, self.model.artists]))

    def draw(self, data):
        # TODO: fix draw order of entities in the scene
        # TODO: add some error checking here
        obstacle_artist = self.obstacles.draw(data["obstacles0"])
        model_artist = self.model.draw(data["model"])
        self.artists = list(itertools.chain.from_iterable([obstacle_artist, model_artist]))
        return self.artists


class Player(FuncAnimation):
    def __init__(self, fig, func, init_func=None, fargs=None, num_frames=100, pos=(0.125, 0.92), **kwargs):
        # initialize class variables from input
        self.current_frame = 0
        self.min =0
        self.max = num_frames - 1
        self.runs = True
        self.forwards = True
        self.fig = fig
        self.func = func

        # setup axes
        replay_ax = self.fig.add_axes([pos[0],pos[1], 0.64, 0.04])
        divider = mpl_toolkits.axes_grid1.make_axes_locatable(replay_ax)
        one_back_ax = divider.append_axes("right", size="80%", pad=0.05)
        back_ax = divider.append_axes("right", size="80%", pad=0.05)
        stop_ax = divider.append_axes("right", size="80%", pad=0.05)
        forward_ax = divider.append_axes("right", size="80%", pad=0.05)
        one_forward_ax = divider.append_axes("right", size="80%", pad=0.05)
        sliderax = divider.append_axes("right", size="500%", pad=0.1)

        # add widgets
        self.button_replay = widgets.Button(replay_ax, label='$\u27F2$')
        self.button_one_back = widgets.Button(one_back_ax, label='$\u29CF$')
        self.button_back = widgets.Button(back_ax, label='$\u25C0$')
        self.button_stop = widgets.Button(stop_ax, label='$\u25A0$')
        self.button_forward = widgets.Button(forward_ax, label='$\u25B6$')
        self.button_one_forward = widgets.Button(one_forward_ax, label='$\u29D0$')
        self.slider = widgets.Slider(sliderax, '', self.min, self.max, valinit=self.current_frame, valfmt="%i")

        # assign functions to widgets
        self.button_replay.on_clicked(self.replay)
        self.button_one_back.on_clicked(self.one_backward)
        self.button_back.on_clicked(self.backward)
        self.button_stop.on_clicked(self.stop)
        self.button_forward.on_clicked(self.forward)
        self.button_one_forward.on_clicked(self.one_forward)
        self.slider.on_changed(self.set_pos)

        # subclass as a FuncAnimation
        FuncAnimation.__init__(self, self.fig, self.update, frames=self.play(), init_func=init_func, fargs=fargs, **kwargs )    

    def play(self):
        while self.runs:
            self.current_frame = self.current_frame + self.forwards - (not self.forwards)
            if self.current_frame > self.min and self.current_frame < self.max:
                yield self.current_frame
            else:
                self.stop()
                yield self.current_frame

    def start(self):
        self.runs=True
        self.event_source.start()

    def stop(self, event=None):
        self.runs = False
        self.event_source.stop()

    def forward(self, event=None):
        self.forwards = True
        self.start()

    def backward(self, event=None):
        self.forwards = False
        self.start()

    def replay(self, event=None):
        self.draw_frame(0)
        self.forward()

    def one_forward(self, event=None):
        self.forwards = True
        self.one_step()

    def one_backward(self, event=None):
        self.forwards = False
        self.one_step()

    def one_step(self):
        if self.current_frame > self.min and self.current_frame < self.max:
            self.draw_frame(self.current_frame+self.forwards - (not self.forwards))
        elif self.current_frame == self.min and self.forwards:
            self.draw_frame(self.current_frame + 1)
        elif self.current_frame == self.max and not self.forwards:
            self.draw_frame(self.current_frame - 1)

    def set_pos(self, val):
        self.slider.val = round(val)
        self.current_frame = self.slider.val
        self.func(self.current_frame)

    def update(self, i):
        self.slider.set_val(i)

    def draw_frame(self, frame):
        self.current_frame = frame
        self.func(self.current_frame)
        self.slider.set_val(self.current_frame)
        self.fig.canvas.draw_idle()


class Video():
    def __init__(self, path, fps=4):
        # load scene from file
        fig = plt.figure()
        ax = Axes3D(fig, auto_add_to_figure=False)
        fig.add_axes(ax)
        with open(path) as file:
            self.frames = json.load(file)
        self.scene = Scene(ax, self.frames[0])

        # plot display ssettings
        fig.canvas.manager.set_window_title('Viam Visualizer')
        plt.grid(False)
        plt.axis('off')
        ax.dist=10
        ax.azim=30
        ax.elev=10
        # TODO: make the plot extents programatic
        ax.set_xlim(-1000, 1000)
        ax.set_ylim(-1000, 1000)
        ax.set_zlim(-200, 1800)
        ax.set_box_aspect(np.ptp([ax.get_xlim(), ax.get_ylim(), ax.get_zlim()], axis=1))

        
        # start video
        Player(fig, self.play, num_frames=len(self.frames), interval=1000/fps)
        plt.show()

    def play(self, frame):
        self.scene.draw(self.frames[frame])


if __name__ == "__main__":
    # if len(sys.argv) != 2:
    #     print("Error: must provide a single .json file to display as a command line argument")
    #     sys.exit(1)
    # video = Video(ax, sys.argv[1])
    video = Video("visualization/temp.json")
