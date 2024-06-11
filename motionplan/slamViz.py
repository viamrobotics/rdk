import open3d as o3d
import numpy as np
import matplotlib.pyplot as plt

# Load PCD file
pcd = o3d.io.read_point_cloud("/Users/ankit.khandelwal@viam.com/Documents/rdk/.artifact/data/slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd")

# Convert point cloud to numpy array
points = np.asarray(pcd.points)

# Plot the point cloud
plt.scatter(points[:,0], points[:,1], s=0.1)
plt.show()