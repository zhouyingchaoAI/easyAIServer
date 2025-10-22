package main

import (
	"fmt"
	"os"
	"strings"
)

// migrate_config.go - 配置迁移工具
// 用于升级旧版本的 config.toml，补全缺失的新字段

func main() {
	if len(os.Args) < 2 {
		fmt.Println("使用方法: go run migrate_config.go <config.toml路径>")
		fmt.Println("示例: go run migrate_config.go ../configs/config.toml")
		os.Exit(1)
	}

	configPath := os.Args[1]
	
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("错误: 无法读取配置文件: %v\n", err)
		os.Exit(1)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	
	// 备份原配置
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		fmt.Printf("错误: 无法创建备份文件: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 已创建备份: %s\n", backupPath)
	
	// 迁移配置
	newLines := migrateFrameExtractorTasks(lines)
	
	// 写入新配置
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		fmt.Printf("错误: 无法写入配置文件: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✓ 配置迁移完成: %s\n", configPath)
	fmt.Println("\n迁移内容:")
	fmt.Println("  - 为所有任务添加 config_status = 'configured'")
	fmt.Println("  - 为所有任务添加 preview_image = ''")
	fmt.Println("\n请检查配置文件，确认无误后重启服务。")
}

func migrateFrameExtractorTasks(lines []string) []string {
	var result []string
	inTask := false
	taskFieldCount := 0
	hasConfigStatus := false
	hasPreviewImage := false
	
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		
		// 检测任务开始
		if trimmed == "[[frame_extractor.tasks]]" {
			inTask = true
			taskFieldCount = 0
			hasConfigStatus = false
			hasPreviewImage = false
			result = append(result, line)
			continue
		}
		
		// 检测任务结束（空行或新section）
		if inTask && (trimmed == "" || strings.HasPrefix(trimmed, "[")) {
			// 在任务结束前补全缺失字段
			if !hasConfigStatus {
				result = append(result, "config_status = 'configured'")
			}
			if !hasPreviewImage {
				result = append(result, "preview_image = ''")
			}
			inTask = false
		}
		
		// 在任务中，检测字段
		if inTask {
			if strings.Contains(line, "=") {
				taskFieldCount++
				// 检查是否已有新字段
				if strings.Contains(line, "config_status") {
					hasConfigStatus = true
				}
				if strings.Contains(line, "preview_image") {
					hasPreviewImage = true
				}
			}
		}
		
		result = append(result, line)
	}
	
	return result
}

