const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');

// see https://github.com/facebook/metro/issues/7#issuecomment-421072314
const installedDependencies = require("./package.json").dependencies;

const aliases = {};
Object.keys(installedDependencies).forEach(dep => {
	aliases[dep] = path.resolve(__dirname, "node_modules", dep);
});
if ("proto" in aliases) {
	throw new Error("proto is already in aliases");
}
aliases["proto"] = path.resolve(__dirname, '../../dist/js/proto');

module.exports = {
	mode: "development",
	entry: {
		control: "./src/control.js"
	},
	devtool: 'inline-source-map',
	module: {
		rules: [
			{
				test: /\.ts$/,
				include: /src|proto/,
				exclude: /node_modules/,
				loader: "ts-loader"
			}
		]
	},
	resolve: {
		alias: aliases,
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
