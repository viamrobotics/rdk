# Test Forcesensor

## How to use

On Raspberry Pi (RPI):
1. SSH into RPI: `ssh -A YOUR_GITHUB_USERNAME@YOUR_RPI_NAME`
2. Clone this repo
3. Navigate into this directory, e.g. `cd core/samples/forcesensor/cmd`
4. Build the executable: `go build -tags=pi cmd.go`
5. Get the IP address from your RPI: `hostname -I` (need it later)

On your computer:
1. Clone this repo
2. Navigate into this directory, e.g. `cd core/samples/forcesensor/cmd`
3. Edit IP address in the `index.html` file to match the IP address of your RPI (retrieved in last step above)
    * Example: `let IP_address = "10.237.115.192"`
4. Open `index.html` in your browser (e.g. doubleclick it, or if on a mac, from terminal type `open index.html`)

Back on the RPI:
1. Run the compiled file `sudo ./cmd`

On your computer:
* Refreshing the page will reset the view & connect to the RPI to retrieve data
