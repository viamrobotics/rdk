# Jetson Camera

## Orin AGX Setup

* Follow instructions to install E-Con Systems [e-CAM20_CUOAGX](https://www.e-consystems.com/nvidia-cameras/jetson-agx-orin-cameras/full-hd-ar0234-color-global-shutter-camera.asp) AR0234 driver.
* Ensure driver has successfully installed. `dmesg | grep ar0234` should return a log that looks like `ar0234 Detected Ar0234 sensor R01_RC1`.
* Connect AR0234 camera module and daughter baord to J509 port located on the bottom of the Developer Kit.