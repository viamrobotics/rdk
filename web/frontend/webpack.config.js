const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');

module.exports = {
	mode: "production",
	entry: {
		control: "./src/control.js",
		control_helpers: "./src/rc/control_helpers.js"
	},
	devtool: 'source-map',
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
		minimize: true,
		minimizer: [new TerserPlugin({
			extractComments: false,
			terserOptions: {
				keep_fnames: true,
				keep_classnames: true,
			},
		})],
	},
};
