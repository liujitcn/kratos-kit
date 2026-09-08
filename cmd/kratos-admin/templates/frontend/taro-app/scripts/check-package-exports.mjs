import { readdir, readFile, stat } from 'node:fs/promises'
import { dirname, extname, relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const workspaceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const coreRoot = resolve(workspaceRoot, 'packages/core')
const uiRoot = resolve(workspaceRoot, 'packages/ui')
const systemRoot = resolve(workspaceRoot, 'packages/modules/system')
const ignoredDirectories = new Set(['.git', '.turbo', 'dist', 'node_modules'])
const sourceExtensions = new Set(['.js', '.mjs', '.ts', '.tsx'])
const allFiles = await collectFiles(workspaceRoot)
const packageFiles = allFiles.filter((file) => file.endsWith(`${sep}package.json`))
const packages = new Map()
const publicPackageVersions = new Map()
const violations = []

for (const packageFile of packageFiles) {
  const metadata = JSON.parse(await readFile(packageFile, 'utf8'))
  if (!metadata.name || !metadata.exports) continue
  if (!metadata.private) {
    publicPackageVersions.set(metadata.name, metadata.version)

  }
  const packageRoot = dirname(packageFile)
  const exportEntries = Object.entries(metadata.exports)
  packages.set(metadata.name, { packageRoot, exportEntries })
  for (const [, value] of exportEntries) {
    for (const target of collectTargets(value)) {
      const prefix = target.includes('*') ? target.slice(0, target.indexOf('*')) : target
      const checkPath = resolve(packageRoot, prefix)
      if (!(await pathExists(checkPath))) {
        throw new Error(`${metadata.name} 导出目标不存在：${target}`)
      }
    }
  }
}

const distinctPublicVersions = new Set(publicPackageVersions.values())
if (distinctPublicVersions.size > 1 || [...distinctPublicVersions].some((version) => !version)) {
  const versions = [...publicPackageVersions]
    .map(([name, version]) => `${name}@${version}`)
    .join('、')
  throw new Error(`Taro 公开包版本必须保持一致：${versions}`)
}

for (const sourceFile of allFiles.filter((file) => sourceExtensions.has(extname(file)))) {
  if (sourceFile.includes(`${sep}src${sep}rpc${sep}`)) continue
  const source = await readFile(sourceFile, 'utf8')
  for (const imported of ts.preProcessFile(source, true, true).importedFiles) {
    const specifier = imported.fileName
    const line = source.slice(0, imported.pos).split('\n').length
    if ((specifier === '@' || specifier.startsWith('@/')) && !isWithin(sourceFile, coreRoot)) {
      violations.push(
        `${format(sourceFile, line)}: 非 core 代码不得使用 core 私有别名 ${specifier}`,
      )
      continue
    }
    if (specifier.startsWith('@system/') && !isWithin(sourceFile, systemRoot)) {
      violations.push(
        `${format(sourceFile, line)}: 非 system 代码不得使用 system 私有别名 ${specifier}`,
      )
      continue
    }
    if (specifier.startsWith('.')) {
      const sourcePackage = findPackageByPath(sourceFile)
      const targetPackage = findPackageByPath(resolve(dirname(sourceFile), specifier))
      if (sourcePackage && targetPackage && sourcePackage !== targetPackage) {
        violations.push(`${format(sourceFile, line)}: 禁止通过相对路径跨包引用 ${specifier}`)
      }
      continue
    }
    const packageName = [...packages.keys()].find(
      (name) => specifier === name || specifier.startsWith(`${name}/`),
    )
    if (!packageName) continue
    const subpath = specifier === packageName ? '.' : `./${specifier.slice(packageName.length + 1)}`
    const exported = packages
      .get(packageName)
      .exportEntries.some(([pattern]) => matchExportPattern(pattern, subpath))
    if (!exported) {
      violations.push(
        `${format(sourceFile, line)}: ${specifier} 未在 ${packageName} exports 中公开`,
      )
    }
  }
}

for (const [packageName, { packageRoot }] of packages) {
  if (packageName === '@liujitcn/kratos-taro-app-core' && packageRoot !== coreRoot) {
    violations.push(`${packageName}: core 包目录异常`)
  }
  if (packageName === '@liujitcn/kratos-taro-app-ui' && packageRoot !== uiRoot) {
    violations.push(`${packageName}: UI 包目录异常`)
  }
  if (packageName === '@liujitcn/kratos-taro-app-system' && packageRoot !== systemRoot) {
    violations.push(`${packageName}: system 包目录异常`)
  }
}

if (violations.length) {
  console.error(
    `Taro package exports 检查失败：\n${violations.map((item) => `  ${item}`).join('\n')}`,
  )
  process.exit(1)
}
console.log(`Taro package exports 检查通过（${[...packages.keys()].join('、')}）`)

async function collectFiles(directory) {
  const files = []
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.isDirectory() && ignoredDirectories.has(entry.name)) continue
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) files.push(...(await collectFiles(path)))
    else files.push(path)
  }
  return files
}

function collectTargets(value) {
  if (typeof value === 'string') return [value]
  if (!value || typeof value !== 'object') return []
  return Object.values(value).flatMap(collectTargets)
}

async function pathExists(path) {
  try {
    await stat(path)
    return true
  } catch {
    return false
  }
}

function matchExportPattern(pattern, subpath) {
  if (!pattern.includes('*')) return pattern === subpath
  const escaped = pattern.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace('\\*', '.+')
  return new RegExp(`^${escaped}$`).test(subpath)
}

function findPackageByPath(file) {
  return [...packages.values()].find(({ packageRoot }) => isWithin(file, packageRoot))
}

function isWithin(file, directory) {
  const path = relative(directory, file)
  return path === '' || (!path.startsWith('..') && !path.startsWith(sep))
}

function format(file, line) {
  return `${relative(workspaceRoot, file)}:${line}`
}
