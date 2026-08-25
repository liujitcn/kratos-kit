package migration

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	migrationVersionFormatHint = "支持格式：纯数字（如 000001）、v0.0.1、v0.0.1-20260511170946、v0.0.1.20260511170946"
	databaseTypeMySQL          = "mysql"
	databaseTypeDoris          = "doris"
)

var migrationVersionPattern = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)(?:[-.]([0-9]{14}))?$`)

// migrationVersion 表示迁移目录的可排序版本。
type migrationVersion struct {
	major     uint64
	minor     uint64
	patch     uint64
	timestamp uint64
}

// migrationAsset 表示一个待执行的版本化迁移脚本。
type migrationAsset struct {
	// version 是解析后的版本，用于排序。
	version migrationVersion
	// versionName 是原始版本目录名称，用于写入迁移记录。
	versionName string
	// databaseType 是脚本适用的真实数据库类型。
	databaseType string
	// dataSource 是目标数据源名称；数据库类型目录下的直系文件使用 default。
	dataSource string
	// upScripts 是按文件名排序的升级脚本集合。
	upScripts []migrationScript
	// downScripts 是按文件名排序的回退脚本集合。
	downScripts []migrationScript
	// description 是描述文件的完整文本内容。
	description string
}

// migrationScript 表示一个待执行的升级 SQL 文件。
type migrationScript struct {
	// name 是 SQL 文件名，用于错误信息。
	name string
	// sql 是 SQL 文件内容。
	sql []byte
}

// migrationTargetFiles 表示一个数据源对应的迁移文件集合。
type migrationTargetFiles struct {
	// databaseType 是脚本适用的真实数据库类型。
	databaseType string
	// dataSource 是目标数据源名称。
	dataSource string
	// path 是数据源文件所在目录。
	path string
	// entries 是数据源目录下的文件项。
	entries []fs.DirEntry
}

// loadMigrationAssets 加载按版本、数据库类型和数据源组织的 SQL 脚本及升级描述。
func loadMigrationAssets(f fs.FS, directory string) ([]migrationAsset, error) {
	var entries []fs.DirEntry
	var err error
	entries, err = fs.ReadDir(f, directory)
	if err != nil {
		return nil, fmt.Errorf("读取迁移资源目录失败: %w", err)
	}
	versions := make(map[migrationVersion]string)
	assets := make([]migrationAsset, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("迁移资源 %s 必须放在版本目录中", entry.Name())
		}
		name := entry.Name()
		var version migrationVersion
		version, err = parseMigrationVersion(name)
		if err != nil {
			return nil, fmt.Errorf("迁移目录 %s 版本无效: %w", name, err)
		}
		if existingName, exists := versions[version]; exists {
			return nil, fmt.Errorf("迁移版本 %s 存在重复目录: %s、%s", name, existingName, name)
		}
		versions[version] = name
		versionPath := path.Join(directory, name)
		var versionEntries []fs.DirEntry
		versionEntries, err = fs.ReadDir(f, versionPath)
		if err != nil {
			return nil, fmt.Errorf("读取迁移版本目录 %s 失败: %w", versionPath, err)
		}
		var targetFiles []migrationTargetFiles
		targetFiles, err = findMigrationTargets(f, versionPath, versionEntries)
		if err != nil {
			return nil, err
		}
		for _, targetFile := range targetFiles {
			var upFileNames []string
			var downFileNames []string
			var descriptionFileNames []string
			upFileNames, downFileNames, descriptionFileNames, err = findMigrationFiles(targetFile.path, targetFile.entries)
			if err != nil {
				return nil, err
			}
			var descriptionBuilder strings.Builder
			for _, descriptionFileName := range descriptionFileNames {
				descriptionPath := path.Join(targetFile.path, descriptionFileName)
				var descriptionContent []byte
				descriptionContent, err = fs.ReadFile(f, descriptionPath)
				if err != nil {
					return nil, fmt.Errorf("读取迁移描述文件 %s 失败: %w", descriptionPath, err)
				}
				descriptionBuilder.Write(descriptionContent)
			}
			var upScripts []migrationScript
			upScripts, err = readMigrationScripts(f, targetFile.path, upFileNames)
			if err != nil {
				return nil, err
			}
			var downScripts []migrationScript
			downScripts, err = readMigrationScripts(f, targetFile.path, downFileNames)
			if err != nil {
				return nil, err
			}
			assets = append(assets, migrationAsset{
				version:      version,
				versionName:  name,
				databaseType: targetFile.databaseType,
				dataSource:   targetFile.dataSource,
				upScripts:    upScripts,
				downScripts:  downScripts,
				description:  descriptionBuilder.String(),
			})
		}
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("迁移目录 %s 未提供任何版本目录", directory)
	}
	slices.SortFunc(assets, func(current, other migrationAsset) int {
		if current.version.less(other.version) {
			return -1
		}
		if other.version.less(current.version) {
			return 1
		}
		if current.databaseType != other.databaseType {
			return strings.Compare(current.databaseType, other.databaseType)
		}
		return strings.Compare(current.dataSource, other.dataSource)
	})
	return assets, nil
}

// findMigrationTargets 按数据库类型和数据源划分版本目录中的迁移文件。
func findMigrationTargets(
	f fs.FS,
	versionPath string,
	entries []fs.DirEntry,
) ([]migrationTargetFiles, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("迁移版本目录 %s 未提供数据库类型目录", versionPath)
	}
	targets := make([]migrationTargetFiles, 0, len(entries)+1)
	var err error
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("迁移版本目录 %s 必须按数据库类型存放脚本，发现直系文件 %s", versionPath, entry.Name())
		}
		databaseType := entry.Name()
		if databaseType != databaseTypeMySQL && databaseType != databaseTypeDoris {
			return nil, fmt.Errorf("迁移版本目录 %s 包含不支持的数据库类型 %s，仅支持 mysql、doris", versionPath, databaseType)
		}
		databasePath := path.Join(versionPath, databaseType)
		var databaseEntries []fs.DirEntry
		databaseEntries, err = fs.ReadDir(f, databasePath)
		if err != nil {
			return nil, fmt.Errorf("读取迁移数据库类型目录 %s 失败: %w", databasePath, err)
		}
		var databaseTargets []migrationTargetFiles
		databaseTargets, err = findDatabaseMigrationTargets(f, databaseType, databasePath, databaseEntries)
		if err != nil {
			return nil, err
		}
		targets = append(targets, databaseTargets...)
	}
	slices.SortFunc(targets, func(current, other migrationTargetFiles) int {
		if current.databaseType != other.databaseType {
			return strings.Compare(current.databaseType, other.databaseType)
		}
		return strings.Compare(current.dataSource, other.dataSource)
	})
	return targets, nil
}

// findDatabaseMigrationTargets 按直系文件和一级子目录划分一个数据库类型下的数据源。
func findDatabaseMigrationTargets(
	f fs.FS,
	databaseType string,
	databasePath string,
	entries []fs.DirEntry,
) ([]migrationTargetFiles, error) {
	targets := make([]migrationTargetFiles, 0, len(entries)+1)
	directEntries := make([]fs.DirEntry, 0, len(entries))
	var err error
	for _, entry := range entries {
		if !entry.IsDir() {
			directEntries = append(directEntries, entry)
			continue
		}
		dataSource := entry.Name()
		if dataSource == DefaultTarget {
			return nil, fmt.Errorf("迁移数据库类型目录 %s 不允许使用 default 子目录，默认数据源请直接放置文件", databasePath)
		}
		targetPath := path.Join(databasePath, dataSource)
		var targetEntries []fs.DirEntry
		targetEntries, err = fs.ReadDir(f, targetPath)
		if err != nil {
			return nil, fmt.Errorf("读取迁移数据源目录 %s 失败: %w", targetPath, err)
		}
		targets = append(targets, migrationTargetFiles{
			databaseType: databaseType,
			dataSource:   dataSource,
			path:         targetPath,
			entries:      targetEntries,
		})
	}
	if len(directEntries) > 0 || len(targets) == 0 {
		targets = append(targets, migrationTargetFiles{
			databaseType: databaseType,
			dataSource:   DefaultTarget,
			path:         databasePath,
			entries:      directEntries,
		})
	}
	slices.SortFunc(targets, func(current, other migrationTargetFiles) int {
		return strings.Compare(current.dataSource, other.dataSource)
	})
	return targets, nil
}

// findMigrationFiles 校验版本目录并返回升级、回退 SQL 与描述文件名。
func findMigrationFiles(versionPath string, entries []fs.DirEntry) ([]string, []string, []string, error) {
	upFileNames := make([]string, 0)
	downFileNames := make([]string, 0)
	descriptionFileNames := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, nil, nil, fmt.Errorf("迁移版本目录 %s 不能包含子目录", versionPath)
		}
		switch {
		case strings.HasSuffix(entry.Name(), ".up.sql"):
			upFileNames = append(upFileNames, entry.Name())
		case strings.HasSuffix(entry.Name(), ".down.sql"):
			downFileNames = append(downFileNames, entry.Name())
		case strings.HasSuffix(entry.Name(), ".md"):
			descriptionFileNames = append(descriptionFileNames, entry.Name())
		default:
			return nil, nil, nil, fmt.Errorf("迁移版本目录 %s 中的文件名无效: %s", versionPath, entry.Name())
		}
	}
	slices.Sort(upFileNames)
	slices.Sort(downFileNames)
	slices.Sort(descriptionFileNames)
	return upFileNames, downFileNames, descriptionFileNames, nil
}

// readMigrationScripts 读取并组装指定目录下的迁移脚本。
func readMigrationScripts(f fs.FS, versionPath string, fileNames []string) ([]migrationScript, error) {
	scripts := make([]migrationScript, 0, len(fileNames))
	for _, fileName := range fileNames {
		filePath := path.Join(versionPath, fileName)
		sqlContent, err := fs.ReadFile(f, filePath)
		if err != nil {
			return nil, fmt.Errorf("读取迁移脚本 %s 失败: %w", filePath, err)
		}
		scripts = append(scripts, migrationScript{
			name: fileName,
			sql:  sqlContent,
		})
	}
	return scripts, nil
}

// parseMigrationVersion 解析迁移版本目录名中的版本号。
func parseMigrationVersion(baseName string) (migrationVersion, error) {
	var err error
	var version64 uint64
	version64, err = strconv.ParseUint(baseName, 10, 64)
	if err == nil && version64 > 0 {
		return migrationVersion{patch: version64}, nil
	}
	matches := migrationVersionPattern.FindStringSubmatch(baseName)
	if matches == nil {
		return migrationVersion{}, fmt.Errorf("目录名 %q 格式无效，%s", baseName, migrationVersionFormatHint)
	}
	var version migrationVersion
	version.major, err = strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return migrationVersion{}, fmt.Errorf("目录名 %q 格式无效，%s", baseName, migrationVersionFormatHint)
	}
	version.minor, err = strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return migrationVersion{}, fmt.Errorf("目录名 %q 格式无效，%s", baseName, migrationVersionFormatHint)
	}
	version.patch, err = strconv.ParseUint(matches[3], 10, 64)
	if err != nil {
		return migrationVersion{}, fmt.Errorf("目录名 %q 格式无效，%s", baseName, migrationVersionFormatHint)
	}
	if matches[4] != "" {
		version.timestamp, err = strconv.ParseUint(matches[4], 10, 64)
		if err != nil {
			return migrationVersion{}, fmt.Errorf("目录名 %q 格式无效，%s", baseName, migrationVersionFormatHint)
		}
	}
	return version, nil
}

// less 判断当前版本是否早于目标版本。
func (version migrationVersion) less(other migrationVersion) bool {
	if version.major != other.major {
		return version.major < other.major
	}
	if version.minor != other.minor {
		return version.minor < other.minor
	}
	if version.patch != other.patch {
		return version.patch < other.patch
	}
	return version.timestamp < other.timestamp
}
