module.exports = {
  "*.{js,jsx,ts,tsx}": ["oxlint --fix --config ./internal/lint-config/oxlint.json", "prettier --write"],
  "{!(package)*.json,*.code-snippets,.!(browserslist)*rc}": ["prettier --write --parser json"],
  "package.json": ["prettier --write"],
  "*.vue": ["oxlint --fix --vue-plugin --config ./internal/lint-config/oxlint.json", "prettier --write", "stylelint --fix"],
  "*.{scss,less,styl,html}": ["stylelint --fix", "prettier --write"],
  "*.md": ["prettier --write"]
};
