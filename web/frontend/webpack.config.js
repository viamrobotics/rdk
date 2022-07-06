const path = require('path');

module.exports = {
	entry: {
		control: './src/main.js',
		control_helpers: './src/rc/control_helpers.js',
	},
	module: {
		rules: [
			{
				test: /\.ts$/,
				include: /src/,
				exclude: /node_modules/,
				loader: 'ts-loader',
			}
		]
	},
	resolve: {
		extensions: ['.ts', '.js'],
	},
	output: {
		path: path.resolve(__dirname, '../runtime-shared/static'),
	},
};
