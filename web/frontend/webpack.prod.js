const TerserPlugin = require('terser-webpack-plugin');
const { merge } = require('webpack-merge');
const common = require('./webpack.config.js');

module.exports = merge(common, {
    mode: 'production',
    devtool: 'source-map',
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
});
