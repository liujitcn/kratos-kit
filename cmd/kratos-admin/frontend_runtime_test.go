package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFrontendRuntimeRegistration 使用真实上游 CLI 验证三端模块语言注册和内置源码扫描。
func TestFrontendRuntimeRegistration(t *testing.T) {
	source := os.Getenv("KRATOS_ADMIN_SOURCE_DIR")
	if source == "" {
		t.Skip("设置 KRATOS_ADMIN_SOURCE_DIR 验证真实前端 CLI 生成链路")
	}
	target, err := createProjectWithOptions(projectOptions{projectName: "runtime-check", frontendModule: "system,order"}, t.TempDir(), func(string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("node", "--input-type=module", "-e", `
import { pathToFileURL } from 'node:url';
const [source, target] = process.argv.slice(1);
const admin = await import(pathToFileURL(source + '/frontend/admin/packages/cli/dist/index.js'));
await admin.createBusinessWorkspace({ projectName: target + '/frontend/admin', moduleNames: ['system', 'order'], kratosProject: true });
for (const [terminal, method] of [['uni-app', 'scaffoldKratosApp'], ['taro-app', 'scaffoldKratosTaroApp']]) {
  const cli = await import(pathToFileURL(source + '/frontend/' + terminal + '/packages/cli/src/index.mjs'));
  cli[method](target + '/frontend/' + terminal, { modules: ['system', 'order'], kratosProject: true });
}
`, source, target)
	var output []byte
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("生成上游前端: %v\n%s", err, output)
	}
	command = exec.Command("python3", "scripts/sync_locales.py", "--write")
	command.Dir = target
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("跨端语言同步失败: %v\n%s", err, output)
	}
	for _, terminal := range []string{"admin", "uni-app", "taro-app"} {
		t.Run(terminal, func(t *testing.T) {
			command := exec.Command("node", "--input-type=module", "-e", `
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import vm from 'node:vm';
import { pathToFileURL } from 'node:url';
const [source, workspace, terminal] = process.argv.slice(1);
const require = createRequire(source + '/frontend/admin/package.json');
const ts = require('typescript');
const locales = ['zh-CN', 'en-US', 'zh-TW', 'ja-JP'];
const base = { name: 'system', views: {}, messages: Object.fromEntries(locales.map(locale => [locale, { 'system.profile.title': 'Profile' }])) };
function load(file, input = readFileSync(file, 'utf8')) {
  if (file.endsWith('.json')) return JSON.parse(readFileSync(file, 'utf8'));
  const code = ts.transpileModule(input.replace(/import\.meta\.glob[^;]+;/g, '({});'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, resolveJsonModule: true, esModuleInterop: true }
  }).outputText;
  const exports = {};
  vm.runInNewContext(code, { exports, require(id) {
    if (id.startsWith('./locales/') || id.startsWith('./') && id.endsWith('.json')) {
      return load(resolve(dirname(file), id.endsWith('.json') || id.endsWith('.mjs') ? id : id + '.ts'));
    }
    return { defineAdminModule: value => value, defineKratosAppModule: value => value,
      defineKratosTaroModule: value => value, systemAdminModule: base, User: {} };
  } }, { filename: file });
  return exports;
}
for (const name of ['system', 'order']) {
  const entry = terminal === 'admin' ? 'module.ts' : terminal === 'uni-app' ? 'index.mjs' : 'index.ts';
  const exports = load(workspace + '/packages/modules/' + name + '/src/' + entry);
  const module = Object.values(exports).find(value => value?.name);
  for (const locale of locales) assert.ok(module.messages?.[locale], module.name + ' 缺少 ' + locale + ' 语言包');
  if (terminal === 'admin' && name === 'system') assert.equal(module.messages['zh-CN']['system.profile.title'], 'Profile');
}
if (terminal === 'admin') {
  const manifest = load(workspace + '/apps/admin/src/module-manifest.ts');
  assert.ok(manifest.adminModulePackages.includes('@liujitcn/kratos-admin-system'), '内置 System 未参与 Vite 源码扫描，User 无法自动导入');
  assert.equal(manifest.adminModuleManifest.filter(item => item.packageName.includes('system')).length, 1);
  assert.ok(manifest.adminModuleOptimizeDependencies.includes('@liujitcn/kratos-admin-system > swagger-ui-dist/swagger-ui-bundle.js'));
  const core = source + '/frontend/admin/packages/core';
  const coreRequire = createRequire(core + '/package.json');
  const { default: AutoImport } = await import(pathToFileURL(coreRequire.resolve('unplugin-auto-import/vite')));
  const systemFile = source + '/frontend/admin/packages/modules/system/src/module.ts';
  const original = readFileSync(systemFile, 'utf8');
  assert.throws(() => load(systemFile, original), /User is not defined/);
  const plugin = AutoImport({ dts: false, include: manifest.adminModulePackages.includes('@liujitcn/kratos-admin-system') ? [/packages\/modules\/system\/src\//] : [],
    imports: [JSON.parse(readFileSync(core + '/build/auto-imports.json', 'utf8'))] });
  const transform = typeof plugin.transform === 'function' ? plugin.transform : plugin.transform.handler;
  const result = await transform.call({}, original, systemFile);
  assert.ok(result?.code, '内置 System 源码未执行自动导入');
  load(systemFile, result.code);
}
`, source, filepath.Join(target, "frontend", terminal), terminal)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("模块运行时注册失败: %v\n%s", err, output)
			}
		})
	}
}
