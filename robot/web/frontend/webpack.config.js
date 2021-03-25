const path = require('path');

// see https://github.com/facebook/metro/issues/7#issuecomment-421072314
const installedDependencies = require("./package.json").dependencies;

const aliases = {};
Object.keys(installedDependencies).forEach(dep => {
  aliases[dep] = path.resolve(__dirname, "node_modules", dep);
});
if ("proto" in aliases) {
	throw new Error("proto is already in aliases");
}
aliases["proto"] = path.resolve(__dirname, '../../../dist/js/proto');

module.exports = {
	mode: "production",
	entry: "./src/client.js",
	resolve: {
		alias: aliases,
	},
};
