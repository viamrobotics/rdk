const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');

module.exports = {
	mode: "development",
	entry: {
		control: "./src/control.js"
	},
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
		path: path.resolve(__dirname, '../../../runtime-shared/static/rdk-api'),
	},
	optimization: {
		minimize: false,
		minimizer: [new TerserPlugin({
			extractComments: false,
		})],
	},
};
