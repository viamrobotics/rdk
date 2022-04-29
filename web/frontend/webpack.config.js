const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');

module.exports = {
	mode: "development",
	entry: {
		control: "./src/control.js",
		control_helpers: "./src/rc/control_helpers.js"
	},
	devtool: 'inline-source-map',
	module: {
		rules: [
			{
				test: /\.ts$/,
				include: /src/,
				exclude: /node_modules/,
				loader: "ts-loader"
			}
		]
	},
	resolve: {
		extensions: [".ts", ".js"]
	},
	output: {
		path: path.resolve(__dirname, '../runtime-shared/static'),
	},
	optimization: {
		minimize: false,
		minimizer: [new TerserPlugin({
			extractComments: false,
		})],
	},
};
