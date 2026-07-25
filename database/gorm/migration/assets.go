package migration

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// migrationAsset 表示一个待执行的版本化迁移脚本。
type migrationAsset struct {
	// version 是解析后的数字版本，用于排序。
	version uint
	// versionName 是原始版本目录名称，用于写入迁移记录。
	versionName string
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

// loadMigrationAssets 加载按版本目录组织的 SQL 脚本和升级描述文件。
func loadMigrationAssets(f fs.FS, directory string) ([]migrationAsset, error) {
	var entries []fs.DirEntry
	var err error
	entries, err = fs.ReadDir(f, directory)
	if err != nil {
		return nil, fmt.Errorf("读取迁移资源目录失败: %w", err)
	}
	versions := make(map[uint]string)
	assets := make([]migrationAsset, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("迁移资源 %s 必须放在版本目录中", entry.Name())
		}
		name := entry.Name()
		var version uint
		version, err = parseMigrationVersion(name)
		if err != nil {
			return nil, fmt.Errorf("迁移目录 %s 版本无效: %w", name, err)
		}
		if existingName, exists := versions[version]; exists {
			return nil, fmt.Errorf("迁移版本 %d 存在重复目录: %s、%s", version, existingName, name)
		}
		versions[version] = name
		versionPath := path.Join(directory, name)
		var versionEntries []fs.DirEntry
		versionEntries, err = fs.ReadDir(f, versionPath)
		if err != nil {
			return nil, fmt.Errorf("读取迁移版本目录 %s 失败: %w", versionPath, err)
		}
		var upFileNames []string
		var downFileNames []string
		var descriptionFileNames []string
		upFileNames, downFileNames, descriptionFileNames, err = findMigrationFiles(versionPath, versionEntries)
		if err != nil {
			return nil, err
		}
		var descriptionBuilder strings.Builder
		for _, descriptionFileName := range descriptionFileNames {
			descriptionPath := path.Join(versionPath, descriptionFileName)
			var descriptionContent []byte
			descriptionContent, err = fs.ReadFile(f, descriptionPath)
			if err != nil {
				return nil, fmt.Errorf("读取迁移描述文件 %s 失败: %w", descriptionPath, err)
			}
			descriptionBuilder.Write(descriptionContent)
		}
		description := descriptionBuilder.String()
		if strings.TrimSpace(description) == "" {
			return nil, fmt.Errorf("迁移版本目录 %s 的描述文件不能为空", versionPath)
		}
		var upScripts []migrationScript
		upScripts, err = readMigrationScripts(f, versionPath, upFileNames)
		if err != nil {
			return nil, err
		}
		var downScripts []migrationScript
		downScripts, err = readMigrationScripts(f, versionPath, downFileNames)
		if err != nil {
			return nil, err
		}
		assets = append(assets, migrationAsset{
			version:     version,
			versionName: name,
			upScripts:   upScripts,
			downScripts: downScripts,
			description: description,
		})
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("迁移目录 %s 未提供任何版本目录", directory)
	}
	sort.Slice(assets, func(index, other int) bool {
		return assets[index].version < assets[other].version
	})
	return assets, nil
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
	sort.Strings(upFileNames)
	sort.Strings(downFileNames)
	sort.Strings(descriptionFileNames)
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
func parseMigrationVersion(baseName string) (uint, error) {
	if baseName == "" {
		return 0, fmt.Errorf("目录名不能为空")
	}
	for _, char := range baseName {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("目录名必须只包含数字")
		}
	}
	var err error
	var version64 uint64
	version64, err = strconv.ParseUint(baseName, 10, 32)
	if err != nil || version64 == 0 {
		return 0, fmt.Errorf("目录名不是有效版本号")
	}
	return uint(version64), nil
}
