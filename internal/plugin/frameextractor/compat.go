package frameextractor

import (
	"easydarwin/internal/conf"
	"fmt"
	"log/slog"
	"strings"
)

// MigrateConfig 自动迁移旧配置，补全缺失的字段
// 确保向后兼容，无需手动修改配置文件
func MigrateConfig(cfg *conf.FrameExtractorConfig, logger *slog.Logger) {
	if cfg == nil {
		return
	}

	if logger == nil {
		logger = slog.Default()
	}

	migrated := 0

	for i := range cfg.Tasks {
		task := &cfg.Tasks[i]
		changed := false

		// 补全 ConfigStatus 字段
		if task.ConfigStatus == "" {
			// 如果任务已启用，默认设置为已配置状态
			// 这样可以让旧配置的任务继续正常工作
			if task.Enabled {
				task.ConfigStatus = "configured"
			} else {
				task.ConfigStatus = "unconfigured"
			}
			changed = true
			logger.Debug("migrated task config_status",
				slog.String("task_id", task.ID),
				slog.String("config_status", task.ConfigStatus))
		}

		// 补全 PreviewImage 字段（留空，由系统自动生成）
		if task.PreviewImage == "" && changed {
			// PreviewImage 可以保持为空，系统会在需要时自动生成
			logger.Debug("task preview_image empty (will be auto-generated)",
				slog.String("task_id", task.ID))
		}

		// 补全 TaskType 字段（旧版本可能没有）
		if task.TaskType == "" {
			if len(cfg.TaskTypes) > 0 {
				task.TaskType = cfg.TaskTypes[0]
			} else {
				task.TaskType = "未分类"
			}
			changed = true
			logger.Debug("migrated task task_type",
				slog.String("task_id", task.ID),
				slog.String("task_type", task.TaskType))
		}

		// 补全 OutputPath 字段（旧版本可能没有）
		if task.OutputPath == "" {
			task.OutputPath = task.ID
			changed = true
			logger.Debug("migrated task output_path",
				slog.String("task_id", task.ID),
				slog.String("output_path", task.OutputPath))
		}

		if changed {
			migrated++
		}
	}

	if migrated > 0 {
		logger.Info("config migration completed",
			slog.Int("migrated_tasks", migrated),
			slog.Int("total_tasks", len(cfg.Tasks)))
	}
}

// ValidateConfig 验证配置的有效性
func ValidateConfig(cfg *conf.FrameExtractorConfig) []string {
	var warnings []string

	if cfg == nil {
		return append(warnings, "配置为空")
	}

	// 检查存储配置
	if cfg.Store != "local" && cfg.Store != "minio" {
		warnings = append(warnings, "store 必须为 'local' 或 'minio'")
	}

	// 检查MinIO配置
	if cfg.Store == "minio" {
		if cfg.MinIO.Endpoint == "" {
			warnings = append(warnings, "使用MinIO存储时，endpoint 不能为空")
		}
		if cfg.MinIO.Bucket == "" {
			warnings = append(warnings, "使用MinIO存储时，bucket 不能为空")
		}
	}

	// 检查任务配置
	for i, task := range cfg.Tasks {
		if task.ID == "" {
			warnings = append(warnings,
				fmt.Sprintf("任务 #%d: id 不能为空", i))
		}
		if task.RtspURL == "" {
			warnings = append(warnings,
				fmt.Sprintf("任务 '%s': rtsp_url 不能为空", task.ID))
		}
		if task.ConfigStatus != "" &&
			task.ConfigStatus != "configured" &&
			task.ConfigStatus != "unconfigured" {
			warnings = append(warnings,
				fmt.Sprintf("任务 '%s': config_status 必须为 'configured' 或 'unconfigured'", task.ID))
		}
		if task.TaskType == "绊线人数统计" && strings.TrimSpace(task.PreferredAlgorithmEndpoint) == "" {
			warnings = append(warnings,
				fmt.Sprintf("任务 '%s': 绊线人数统计任务必须设置 preferred_algorithm_endpoint", task.ID))
		}
	}

	return warnings
}
