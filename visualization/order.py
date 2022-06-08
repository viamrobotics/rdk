#! /usr/bin/python
import numpy as np
import matplotlib
from matplotlib import pyplot as plt
from matplotlib import animation
import random
matplotlib.use('TKAgg')

def init():
    imobj.set_data(np.zeros((100, 100)))
    line_simu.set_data([], [])
    time_text.set_text('time = 0.0')

    return imobj , line_simu,  time_text,  l

def animate(i):
    global data
    imobj.set_data( data + np.random.random((100,1)) * 0.5 )

    imobj.set_zorder(0)
    y_simu = np.linspace(-100,-10, 100)
    x_simu = np.linspace(-10, 10, 100) 
    line_simu.set_data(x_simu, y_simu)
    time_text.set_text('time = %.1f' % i )


    return imobj , line_simu, time_text,  l


def forceAspect(ax,aspect=1):
    im = ax.get_images()
    extent =  im[0].get_extent()
    ax.set_aspect(abs((extent[1]-extent[0])/(extent[3]-extent[2]))/aspect)

fig = plt.figure()
ax = plt.axes(xlim=(-15,15), ylim=(-110, 0) , aspect=1)

data = np.random.random((100,100)) - .5
imobj = ax.imshow( data , extent=[-15,15, -110, 0.0], origin='lower', cmap=plt.cm.gray, vmin=-2, vmax=2, alpha=1.0, zorder=1, aspect=1)

line_simu, = ax.plot([], [],"r--", lw=2, markersize=4 , label = "Some curve" ,  zorder= 1 )
time_text = ax.text(-14.0, -108, '', zorder=10)

forceAspect(ax,aspect=1)

l = plt.legend(loc='lower right', prop={'size':8} )

anim = animation.FuncAnimation(fig, animate, init_func=init,  frames=range( 50), interval=500, blit=True)

plt.show()