import numpy as np
import matplotlib.widgets
import mpl_toolkits.axes_grid1
import matplotlib.pyplot as plt
from matplotlib.animation import FuncAnimation

class Player(FuncAnimation):
    def __init__(self, fig, func, frames=None, init_func=None, fargs=None,
                 save_count=None, mini=0, maxi=100, pos=(0.125, 0.92), **kwargs):
        self.current_frame = 0
        self.min=mini
        self.max=maxi
        self.runs = True
        self.forwards = True
        self.fig = fig
        self.func = func

        # setup axes
        one_back_ax = self.fig.add_axes([pos[0],pos[1], 0.64, 0.04])
        divider = mpl_toolkits.axes_grid1.make_axes_locatable(one_back_ax)
        back_ax = divider.append_axes("right", size="80%", pad=0.05)
        stop_ax = divider.append_axes("right", size="80%", pad=0.05)
        forward_ax = divider.append_axes("right", size="80%", pad=0.05)
        ofax = divider.append_axes("right", size="100%", pad=0.05)
        sliderax = divider.append_axes("right", size="500%", pad=0.07)

        # add widgets
        self.button_one_back = matplotlib.widgets.Button(one_back_ax, label='$\u29CF$')
        self.button_back = matplotlib.widgets.Button(back_ax, label='$\u25C0$')
        self.button_stop = matplotlib.widgets.Button(stop_ax, label='$\u25A0$')
        self.button_forward = matplotlib.widgets.Button(forward_ax, label='$\u25B6$')
        self.button_one_forward = matplotlib.widgets.Button(ofax, label='$\u29D0$')
        self.slider = matplotlib.widgets.Slider(sliderax, '', self.min, self.max, valinit=self.current_frame)

        # assign functions to widgets
        self.button_one_back.on_clicked(self.one_backward)
        self.button_back.on_clicked(self.backward)
        self.button_stop.on_clicked(self.stop)
        self.button_forward.on_clicked(self.forward)
        self.button_one_forward.on_clicked(self.one_forward)
        self.slider.on_changed(self.set_pos)

        # subclass as a FuncAnimation
        FuncAnimation.__init__(self,self.fig, self.update, frames=self.play(), 
                                           init_func=init_func, fargs=fargs,
                                           save_count=save_count, **kwargs )    

    def play(self):
        while self.runs:
            self.current_frame = self.current_frame+self.forwards-(not self.forwards)
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

    def one_forward(self, event=None):
        self.forwards = True
        self.one_step()

    def one_backward(self, event=None):
        self.forwards = False
        self.one_step()

    def one_step(self):
        if self.current_frame > self.min and self.current_frame < self.max:
            self.current_frame = self.current_frame+self.forwards-(not self.forwards)
        elif self.current_frame == self.min and self.forwards:
            self.current_frame+=1
        elif self.current_frame == self.max and not self.forwards:
            self.current_frame-=1
        self.func(self.current_frame)
        self.slider.set_val(self.current_frame)
        self.fig.canvas.draw_idle()

    def set_pos(self, i):
        self.current_frame = int(self.slider.val)
        self.func(self.current_frame)

    def update(self, i):
        self.slider.set_val(i)


### using this class is as easy as using FuncAnimation:            

fig, ax = plt.subplots()
x = np.linspace(0,6*np.pi, num=100)
y = np.sin(x)

ax.plot(x,y)
point, = ax.plot([],[], marker="o", color="crimson", ms=15)

def update(i):
    point.set_data(x[i],y[i])

ani = Player(fig, update, maxi=len(y)-1)

plt.show()