import pygame
import sys
import math
import json
# Initialize pygame
pygame.init()

# Set up the screen
screen_width, screen_height = 800, 600
screen = pygame.display.set_mode((screen_width, screen_height))
pygame.display.set_caption("Drawing Shapes")

# Colors
WHITE = (255, 255, 255)
BLACK = (0, 0, 0)
RED = (255, 0, 0)
GREEN = (0, 255, 0)
BLUE = (0, 0, 255)

rectangle_obstacles = {}
rectangle_count = 0
circle_obstacles = {}
circle_count = 0

# Function to draw rectangle with adjustable size
def draw_resizable_rect(start_pos, end_pos):
    rect_width = end_pos[0] - start_pos[0]
    rect_height = end_pos[1] - start_pos[1]
    pygame.draw.rect(screen, RED, (start_pos[0], start_pos[1], rect_width, rect_height), 2)

def draw_resizable_circle(center, radius):
    pygame.draw.circle(screen, RED, center, radius, 2)

# Main loop
running = True
drawing_shape = None
mouse_down = False
start_pos = None
# Fill the screen with white color
screen.fill(WHITE)
while running:
    for event in pygame.event.get():
        if event.type == pygame.QUIT:
            running = False
        elif event.type == pygame.KEYDOWN:
            if event.key == pygame.K_r:  # Press 'r' to draw rectangles
                drawing_shape = 'rectangle'
            elif event.key == pygame.K_c:  # Press 'c' to draw circles
                drawing_shape = 'circle'
        elif event.type == pygame.MOUSEBUTTONDOWN:
            if event.button == 1:  # Left mouse button
                mouse_down = True
                start_pos = event.pos
        elif event.type == pygame.MOUSEBUTTONUP:
            if event.button == 1:  # Left mouse button
                mouse_down = False
                if drawing_shape == 'rectangle':
                    # Draw final rectangle
                    draw_resizable_rect(start_pos, end_pos)
                    to_add_start = (start_pos[0] * 5, start_pos[1] * 5)
                    to_add_end = (end_pos[0] * 5, end_pos[1] * 5)
                    rectangle_obstacles[to_add_start] = to_add_end
                    rectangle_count += 1
                    start_pos = None
                else:
                    # draw final circle
                    radius = math.sqrt((start_pos[0] - end_pos[0])**2 + (start_pos[1] - end_pos[1])**2)
                    draw_resizable_circle(start_pos, radius)
                    new_radius = radius = math.sqrt((start_pos[0]*5 - end_pos[0]*5)**2 + (start_pos[1]*5 - end_pos[1]*5)**2)
                    to_add = (start_pos[0] * 5, start_pos[1] * 5)
                    circle_obstacles[to_add] = new_radius
                    circle_count += 1
                    start_pos = None
        elif event.type == pygame.MOUSEMOTION:
            if mouse_down and drawing_shape == 'rectangle':
                # Draw rectangle while dragging
                end_pos = event.pos
                draw_resizable_rect(start_pos, end_pos)

            elif mouse_down and drawing_shape == 'circle':
                end_pos = event.pos
                radius = math.sqrt((start_pos[0] - end_pos[0])**2 + (start_pos[1] - end_pos[1])**2 )
                draw_resizable_circle(start_pos, radius)
    # Update the display
    pygame.display.update()

# Quit pygame
print(rectangle_obstacles)
print(circle_obstacles)
with open("customObstacles.txt", 'w') as obstacleWriter:
    for start_pos, end_pos in rectangle_obstacles.items():
        obstacleWriter.write(f"{start_pos[0]}:{start_pos[1]}:{end_pos[0]}:{end_pos[1]}\n")
    obstacleWriter.write("\n")
    for start_pos, radius in circle_obstacles.items():
        obstacleWriter.write(f"{start_pos[0]}:{start_pos[1]}:{radius}\n")

pygame.quit()
sys.exit()
