const TerserPlugin = require('terser-webpack-plugin');
const { merge } = require('webpack-merge');
const common = require('./webpack.config.js');

module.exports = merge(common, {
    mode: 'development',
    devtool: 'inline-source-map',
    optimization: {
		minimize: false,
		minimizer: [new TerserPlugin({
			extractComments: false,
		})],
	},
});
