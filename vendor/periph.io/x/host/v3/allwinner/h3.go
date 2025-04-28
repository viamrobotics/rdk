// Copyright 2022 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// This file contains pin mapping information that is specific to the Allwinner
// H2+ and H3 model.

package allwinner

import (
	"strconv"
	"strings"

	"periph.io/x/conn/v3/pin"
	"periph.io/x/host/v3/sysfs"
)

// mappingH3 describes the mapping of the H3 processor GPIO pins to their
// functions. The mappings source is the official H3 Datasheet, version 1.0,
// page 74 (chapter 3.2 GPIO Multiplexing Functions).
// http://dl.linux-sunxi.org/H3/Allwinner_H3_Datasheet_V1.0.pdf
var mappingH3 = map[string][5]pin.Func{
	"PA0":  {"UART2_TX", "JTAG_MS", "", "", "PA_EINT0"},
	"PA1":  {"UART2_RX", "JTAG_CK", "", "", "PA_EINT1"},
	"PA2":  {"UART2_RTS", "JTAG_DO", "", "", "PA_EINT2"},
	"PA3":  {"UART2_CTS", "JTAG_DI", "", "", "PA_EINT3"},
	"PA4":  {"UART0_TX", "", "", "", "PA_EINT4"},
	"PA5":  {"UART0_RX", "PWM0", "", "", "PA_EINT5"},
	"PA6":  {"SIM_PWREN", "PWM1", "", "", "PA_EINT6"},
	"PA7":  {"SIM_CK", "", "", "", "PA_EINT7"},
	"PA8":  {"SIM_DATA", "", "", "", "PA_EINT8"},
	"PA9":  {"SIM_RST", "", "", "", "PA_EINT9"},
	"PA10": {"SIM_DET", "", "", "", "PA_EINT10"},
	"PA11": {"TWI0_SCK", "DI_TX", "", "", "PA_EINT11"},
	"PA12": {"TWI0_SDA", "DI_RX", "", "", "PA_EINT12"},
	"PA13": {"SPI1_CS", "UART3_TX", "", "", "PA_EINT13"},
	"PA14": {"SPI1_CLK", "UART3_RX", "", "", "PA_EINT14"},
	"PA15": {"SPI1_MOSI", "UART3_RTS", "", "", "PA_EINT15"},
	"PA16": {"SPI1_MOSI", "UART3_CTS", "", "", "PA_EINT16"},
	"PA17": {"OWA_OUT", "", "", "", "PA_EINT17"},
	"PA18": {"PCM0_SYNC", "TWI1_SCK", "", "", "PA_EINT18"},
	"PA19": {"PCM0_CLK", "TWI1_SDA", "", "", "PA_EINT19"},
	"PA20": {"PCM0_DOUT", "SIM_VPPEN", "", "", "PA_EINT20"},
	"PA21": {"PCM0_DIN", "SIM_VPPPP", "", "", "PA_EINT21"},

	"PC0":  {"NAND_WE", "SPI0_MOSI"},
	"PC1":  {"NAND_ALE", "SPI0_MISO"},
	"PC2":  {"NAND_CLE", "SPI0_CLK"},
	"PC3":  {"NAND_CE1", "SPI0_CS"},
	"PC4":  {"NAND_CE0"},
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

	"PD0":  {"RGMII_RXD3"},
	"PD1":  {"RGMII_RXD2"},
	"PD2":  {"RGMII_RXD1"},
	"PD3":  {"RGMII_RXD0"},
	"PD4":  {"RGMII_RXCK"},
	"PD5":  {"RGMII_RXCTL"},
	"PD6":  {"RGMII_NULL"},
	"PD7":  {"RGMII_TXD3"},
	"PD8":  {"RGMII_TXD2"},
	"PD9":  {"RGMII_TXD1"},
	"PD10": {"RGMII_TXD0"},
	"PD11": {"RGMII_NULL"},
	"PD12": {"RGMII_TXCK"},
	"PD13": {"RGMII_TXCTL"},
	"PD14": {"RGMII_NULL"},
	"PD15": {"RGMII_CLKIN"},
	"PD16": {"MDC"},
	"PD17": {"MDIO"},

	"PE0":  {"CSI_PCLK", "TS_CLK"},
	"PE1":  {"CSI_MCLK", "TS_ERR"},
	"PE2":  {"CSI_HSYNC", "TS_SYNC"},
	"PE3":  {"CSI_VSYNC", "TS_DVLD"},
	"PE4":  {"CSI_D0", "TS_D0"},
	"PE5":  {"CSI_D1", "TS_D1"},
	"PE6":  {"CSI_D2", "TS_D2"},
	"PE7":  {"CSI_D3", "TS_D3"},
	"PE8":  {"CSI_D4", "TS_D4"},
	"PE9":  {"CSI_D5", "TS_D5"},
	"PE10": {"CSI_D6", "TS_D6"},
	"PE11": {"CSI_D7", "TS_D7"},
	"PE12": {"CSI_SCK", "TWI2_SCK"},
	"PE13": {"CSI_SDA", "TWI2_SDA"},
	"PE14": {""},
	"PE15": {""},

	"PF0": {"SDC0_D1", "JTAG_MS"},
	"PF1": {"SDC0_D0", "JTAG_DI"},
	"PF2": {"SDC0_CLK", "UART0_TX"},
	"PF3": {"SDC0_CMD", "JTAG_DO"},
	"PF4": {"SDC0_D3", "UART0_RX"},
	"PF5": {"SDC0_D2", "JTAG_CK"},
	"PF6": {"SDC0_DET"},

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

	"PL0":  {"S_TWI_SCK", "", "", "", "S_PL_EINT0"},
	"PL1":  {"S_TWI_SDA", "", "", "", "S_PL_EINT1"},
	"PL2":  {"S_UART_TX", "", "", "", "S_PL_EINT2"},
	"PL3":  {"S_UART_RX", "", "", "", "S_PL_EINT3"},
	"PL4":  {"S_JTAG_MS", "", "", "", "S_PL_EINT4"},
	"PL5":  {"S_JTAG_CK", "", "", "", "S_PL_EINT5"},
	"PL6":  {"S_JTAG_DO", "", "", "", "S_PL_EINT6"},
	"PL7":  {"S_JTAG_DI", "", "", "", "S_PL_EINT7"},
	"PL8":  {"", "", "", "", "S_PL_EINT8"},
	"PL9":  {"", "", "", "", "S_PL_EINT9"},
	"PL10": {"S_PWM", "", "", "", "S_PL_EINT10"},
	"PL11": {"S_CIR_RX", "", "", "", "S_PL_EINT12"},
}

// mapH3Pins uses mappingH3 to set the altFunc fields of all the GPIO pings and
// mark them as available. This is called if the generic allwinner processor
// code detects a H2+ or H3 processor.
func mapH3Pins() error {
	for name, altFuncs := range mappingH3 {
		if strings.HasPrefix(name, "PL") {
			pinNumStr := name[2:]
			pinNum, err := strconv.Atoi(pinNumStr)
			if err != nil {
				return err
			}
			pin := &cpuPinsPL[pinNum]
			pin.available = true

			// Initializes the sysfs corresponding pin right away.
			pin.sysfsPin = sysfs.Pins[pin.Number()]
		} else {
			pin := cpupins[name]
			pin.altFunc = altFuncs
			pin.available = true
			if strings.Contains(string(altFuncs[4]), "_EINT") ||
				strings.Contains(string(altFuncs[3]), "_EINT") {
				pin.supportEdge = true
			}
			// Initializes the sysfs corresponding pin right away.
			pin.sysfsPin = sysfs.Pins[pin.Number()]
		}
	}
	return nil
}
