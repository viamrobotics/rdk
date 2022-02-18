const path = require('path')
const controlNodeModules = path.resolve(__dirname, './node_modules')
const dlsNodeModules = path.resolve(__dirname, '../dls/node_modules')
const rdkNodeModules = path.resolve(__dirname, '../rdk-api/node_modules')

const controlSrc = path.resolve(__dirname, 'src')
const dlsSrc = path.resolve(__dirname, '../dls/src')
const rdkSrc = path.resolve(__dirname, '../rdk-api/src')

module.exports = {
  // pluginOptions: {
  //   i18n: {
  //     locale: "en",
  //     fallbackLocale: "en",
  //     localeDir: "src/i18n",
  //     enableInSFC: true,
  //     includeLocales: false,
  //     enableBridge: true,
  //   },
  // },
  css: {
    extract: {
      filename: `controlApp.css`,
      chunkFilename: `controlApp.css`,
    },
  },
  configureWebpack: {
    output: {
      filename: 'controlApp.js',
    },
    resolve: {
        modules: [controlNodeModules, dlsNodeModules, rdkNodeModules],
        alias: {
            '@dls': dlsSrc,
            '@control': controlSrc,
            '@rdk': rdkSrc
        }
    }
  },
  chainWebpack: config => {
    config.optimization.delete('splitChunks')
  }
};
