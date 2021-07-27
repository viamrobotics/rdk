# Test Forcesensor

## How to use

On Raspberry Pi (RPI):
1. SSH into RPI: `ssh YOUR_GITHUB_USERNAME@YOUR_RPI_NAME`
2. Clone this repo
3. Get the IP address from your RPI: `hostname -I` (need it later)
4. Navigate into this directory, e.g. `cd core/samples/forcesensor/cmd`
5. Build the executable: `go build -tags=pi cmd.go`
6. Run it `./cmd`

On separate computer:
1. Clone this repo
2. Navigate into this directory, e.g. `cd core/samples/forcesensor/cmd`
3. Edit IP address in the `index.html` file to match the IP address of your RPI (retrieved in steps above)
    * Example: `let IP_address = "10.237.115.192"`
4. Open `index.html` in your browser (e.g. doubleclick it)