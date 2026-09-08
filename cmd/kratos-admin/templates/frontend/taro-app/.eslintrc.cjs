module.exports = {
  root: true,
  extends: ['taro/react'],
  parserOptions: {
    project: false,
    requireConfigFile: false,
  },
  rules: {
    'react/react-in-jsx-scope': 'off',
  },
  ignorePatterns: [
    '**/dist/**',
    '**/src/rpc/**',
    'node_modules/**',
    'apps/taro-app/src/pages/**',
    '!apps/taro-app/src/pages/bootstrap/**',
    'apps/taro-app/src/pagesMember/**',
  ],
}
