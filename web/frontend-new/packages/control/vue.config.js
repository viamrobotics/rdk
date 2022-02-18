const path = require('path')
const controlNodeModules = path.resolve(__dirname, './node_modules')
const dlsNodeModules = path.resolve(__dirname, '../dls/node_modules')

const controlSrc = path.resolve(__dirname, 'src')
const dlsSrc = path.resolve(__dirname, '../dls/src')

module.exports = {
  pluginOptions: {
    i18n: {
      locale: "en",
      fallbackLocale: "en",
      localeDir: "i18n",
      enableInSFC: true,
      includeLocales: false,
      enableBridge: true,
    },
  },
  configureWebpack: {
    output: {
      filename: 'bundle.js',
    },
    resolve: {
        modules: [controlNodeModules, dlsNodeModules],
        alias: {
            '@dls': dlsSrc,
            '@control': controlSrc
        }
    }
  }
};
