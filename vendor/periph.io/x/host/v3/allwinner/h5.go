// Copyright 2022 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// This file contains pin mapping information that is specific to the Allwinner
// H5 model.

package allwinner

import (
	"strings"

	"periph.io/x/conn/v3/pin"
	"periph.io/x/host/v3/sysfs"
)

// mappingH5 describes the mapping of the H5 processor gpios to their
// alternate functions.
//
// It omits the in & out functions which are available on all gpio.
//
// The mapping comes from the datasheet page 55:
// https://linux-sunxi.org/images/a/a3/Allwinner_H5_Manual_v1.0.pdf
//
//   - The datasheet uses TWI instead of I2C but it is renamed here for
//     consistency.
//   - RGMII means Reduced gigabit media-independent interface.
//   - SDC means SDCard?
//   - NAND connects to a NAND flash controller.
//   - CSI and CCI are for video capture.
var mappingH5 = map[string][5]pin.Func{
	"PA0":  {"UART2_TX", "JTAG_MS", "", "", "PA_EINT0"},
	"PA1":  {"UART2_RX", "JTAG_CK", "", "", "PA_EINT1"},
	"PA2":  {"UART2_RTS", "JTAG_DO", "", "", "PA_EINT2"},
	"PA3":  {"UART2_CTS", "JTAG_DI", "", "", "PA_EINT3"},
	"PA4":  {"UART0_TX", "", "", "", "PA_EINT4"},
	"PA5":  {"UART0_RX", "PWM0", "", "", "PA_EINT5"},
	"PA6":  {"SIM0_PWREN", "PCM0_MCLK", "", "", "PA_EINT6"},
	"PA7":  {"SIM0_CLK", "", "", "", "PA_EINT7"},
	"PA8":  {"SIM0_DATA", "", "", "", "PA_EINT8"},
	"PA9":  {"SIM0_RST", "", "", "", "PA_EINT9"},
	"PA10": {"SIM0_DET", "", "", "", "PA_EINT10"},
	"PA11": {"I2C0_SCK", "DI_TX", "", "", "PA_EINT11"},
	"PA12": {"I2C0_SDA", "DI_RX", "", "", "PA_EINT12"},
	"PA13": {"SPI1_CS", "UART3_TX", "", "", "PA_EINT13"},
	"PA14": {"SPI1_CLK", "UART3_RX", "", "", "PA_EINT14"},
	"PA15": {"SPI1_MOSI", "UART3_RTS", "", "", "PA_EINT15"},
	"PA16": {"SPI1_MISO", "UART3_CTS", "", "", "PA_EINT16"},
	"PA17": {"OWA_OUT", "", "", "", "PA_EINT17"},
	"PA18": {"PCM0_SYNC", "I2C1_SCK", "", "", "PA_EINT18"},
	"PA19": {"PCM0_CLK", "I2C1_SDA", "", "", "PA_EINT19"},
	"PA20": {"PCM0_DOUT", "SIM0_VPPEN", "", "", "PA_EINT20"},
	"PA21": {"PCM0_DIN", "SIM0_VPPPP", "", "", "PA_EINT21"},

	"PC0":  {"NAND_WE", "SPI0_MOSI"},
	"PC1":  {"NAND_ALE", "SPI0_MISO", "SDC2_DS"},
	"PC2":  {"NAND_CLE", "SPI0_CLK"},
	"PC3":  {"NAND_CE1", "SPI0_CS"},
	"PC4":  {"NAND_CE0", "", "SPI0_MISO"},
	"PC5":  {"NAND_RE", "SDC2_CLK"},
	"PC6":  {"NAND_RB0", "SDC2_CMD"},
	"PC7":  {"NAND_RB1"},
	"PC8":  {"NAND_DQ0", "SDC2_D0"},
	"PC9":  {"NAND_DQ1", "SDC2_D1"},
	"PC10": {"NAND_DQ2", "SDC2_D2"},
	"PC11": {"NAND_DQ3", "SDC2_D3"},
	"PC12": {"NAND_DQ4", "SDC2_D4"},
	"PC13": {"NAND_DQ5", "SDC2_D5"},
	"PC14": {"NAND_DQ6", "SDC2_D6"},
	"PC15": {"NAND_DQ7", "SDC2_D7"},
	"PC16": {"NAND_DQS", "SDC2_RST"},

	"PD0":  {"RGMII_RXD3", "DI_TX", "TS2_CLK"},
	"PD1":  {"RGMII_RXD2", "DI_RX", "TS2_ERR"},
	"PD2":  {"RGMII_RXD1", "", "TS2_SYNC"},
	"PD3":  {"RGMII_RXD0", "", "TS2_DVLD"},
	"PD4":  {"RGMII_RXCK", "", "TS2_D0"},
	"PD5":  {"RGMII_RXCTL", "", "TS2_D1"},
	"PD6":  {"RGMII_NULL", "", "TS2_D2"},
	"PD7":  {"RGMII_TXD3", "", "TS2_D3", "TS3_CLK"},
	"PD8":  {"RGMII_TXD2", "", "TS2_D4", "TS3_ERR"},
	"PD9":  {"RGMII_TXD1", "", "TS2_D5", "TS3_SYNC"},
	"PD10": {"RGMII_TXD0", "", "TS2_D6", "TS3_DVLD"},
	"PD11": {"RGMII_NULL", "", "TS2_D7", "TS3_D0"},
	"PD12": {"RGMII_TXCK", "", "SIM1_PWREN"},
	"PD13": {"RGMII_TXCTL", "", "SIM1_CLK"},
	"PD14": {"RGMII_NULL", "", "SIM1_DATA"},
	"PD15": {"RGMII_CLKIN", "", "SIM1_RST"},
	"PD16": {"MDC", "", "SIM1_DET"},
	"PD17": {"MDIO"},

	"PE0":  {"CSI_PCLK", "TS0_CLK"},
	"PE1":  {"CSI_MCLK", "TS0_ERR"},
	"PE2":  {"CSI_HSYNC", "TS0_SYNC"},
	"PE3":  {"CSI_VSYNC", "TS0_DVLD"},
	"PE4":  {"CSI_D0", "TS0_D0"},
	"PE5":  {"CSI_D1", "TS0_D1"},
	"PE6":  {"CSI_D2", "TS0_D2"},
	"PE7":  {"CSI_D3", "TS0_D3", "TS1_CLK"},
	"PE8":  {"CSI_D4", "TS0_D4", "TS1_ERR"},
	"PE9":  {"CSI_D5", "TS0_D5", "TS1_SYNC"},
	"PE10": {"CSI_D6", "TS0_D6", "TS1_DVLD"},
	"PE11": {"CSI_D7", "TS0_D7", "TS1_D0"},
	"PE12": {"CSI_SCK", "I2C2_SCK"},
	"PE13": {"CSI_SDA", "I2C2_SDA"},
	"PE14": {"", "SIM1_VPPEN"},
	"PE15": {"", "SIM1_VPPPP"},

	"PF0": {"SDC0_D1", "JTAG_MS", "", "", "PF_EINT0"},
	"PF1": {"SDC0_D0", "JTAG_DI", "", "", "PF_EINT1"},
	"PF2": {"SDC0_CLK", "UART0_TX", "", "", "PF_EINT2"},
	"PF3": {"SDC0_CMD", "JTAG_DO", "", "", "PF_EINT3"},
	"PF4": {"SDC0_D3", "UART0_RX", "", "", "PF_EINT4"},
	"PF5": {"SDC0_D2", "JTAG_CK", "", "", "PF_EINT5"},
	"PF6": {"", "", "", "", "PF_EINT6"},

	"PG0":  {"SDC1_CLK", "", "", "", "PG_EINT0"},
	"PG1":  {"SDC1_CMD", "", "", "", "PG_EINT1"},
	"PG2":  {"SDC1_D0", "", "", "", "PG_EINT2"},
	"PG3":  {"SDC1_D1", "", "", "", "PG_EINT3"},
	"PG4":  {"SDC1_D2", "", "", "", "PG_EINT4"},
	"PG5":  {"SDC1_D3", "", "", "", "PG_EINT5"},
	"PG6":  {"UART1_TX", "", "", "", "PG_EINT6"},
	"PG7":  {"UART1_RX", "", "", "", "PG_EINT7"},
	"PG8":  {"UART1_RTS", "", "", "", "PG_EINT8"},
	"PG9":  {"UART1_CTS", "", "", "", "PG_EINT9"},
	"PG10": {"PCM1_SYNC", "", "", "", "PG_EINT10"},
	"PG11": {"PCM1_CLK", "", "", "", "PG_EINT11"},
	"PG12": {"PCM1_DOUT", "", "", "", "PG_EINT12"},
	"PG13": {"PCM1_DIN", "", "", "", "PG_EINT13"},
}

// mapH5Pins uses mappingH5 to actually set the altFunc fields of all gpio
// and mark them as available.
//
// It is called by the generic allwinner processor code if an H5 is detected.
func mapH5Pins() error {
	for name, altFuncs := range mappingH5 {
		pin := cpupins[name]
		pin.altFunc = altFuncs
		pin.available = true
		if strings.Contains(string(altFuncs[4]), "_EINT") {
			pin.supportEdge = true
		}

		// Initializes the sysfs corresponding pin right away.
		pin.sysfsPin = sysfs.Pins[pin.Number()]
	}
	return nil
}
