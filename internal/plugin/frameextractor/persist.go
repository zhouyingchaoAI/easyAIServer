package frameextractor

import (
	"fmt"
	"os"
	"strings"
)

// saveConfigToFile writes current tasks back to config.toml
func (s *Service) saveConfigToFile(cfgPath string) error {
	if cfgPath == "" {
		return nil // skip if no path provided
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	content := string(data)
	
	// find [frame_extractor] section and rebuild
	lines := strings.Split(content, "\n")
	var out []string
	inSection := false
	inMinioSection := false
	skipTask := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// detect section start
		if trimmed == "[frame_extractor]" {
			inSection = true
			out = append(out, line)
			continue
		}
		
		// detect minio subsection
		if trimmed == "[frame_extractor.minio]" {
			inMinioSection = true
			continue
		}
		
		// skip minio lines until next section
		if inMinioSection {
			if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[frame_extractor.minio") {
				inMinioSection = false
			} else {
				continue
			}
		}
		
		// detect next section or task array start
		if inSection && (strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[frame_extractor.tasks]]")) {
			if strings.HasPrefix(trimmed, "[[frame_extractor.tasks]]") {
				skipTask = true
				continue
			} else {
				// end of frame_extractor section, append config and tasks before next section
				out = append(out, s.buildConfigLines()...)
				out = append(out, s.buildTaskLines()...)
				inSection = false
				skipTask = false
				out = append(out, line)
				continue
			}
		}
		
		// skip config lines in frame_extractor section (will rebuild)
		if inSection && !skipTask {
			if strings.Contains(line, "enable") || strings.Contains(line, "interval_ms") || 
			   strings.Contains(line, "output_dir") || strings.Contains(line, "store") {
				continue
			}
		}
		
		// skip old task definitions
		if skipTask {
			if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[frame_extractor") {
				// new section, stop skipping
				skipTask = false
				out = append(out, s.buildConfigLines()...)
				out = append(out, s.buildTaskLines()...)
				out = append(out, line)
			}
			continue
		}
		
		out = append(out, line)
	}
	
	// if still in section at EOF, append config and tasks
	if inSection || skipTask {
		out = append(out, s.buildConfigLines()...)
		out = append(out, s.buildTaskLines()...)
	}
	
	return os.WriteFile(cfgPath, []byte(strings.Join(out, "\n")), 0644)
}

func (s *Service) buildConfigLines() []string {
	var lines []string
	lines = append(lines, fmt.Sprintf("enable = %t", s.cfg.Enable))
	lines = append(lines, fmt.Sprintf("interval_ms = %d", s.cfg.IntervalMs))
	lines = append(lines, fmt.Sprintf("output_dir = '%s'", s.cfg.OutputDir))
	lines = append(lines, fmt.Sprintf("store = '%s'", s.cfg.Store))
	lines = append(lines, "")
	lines = append(lines, "[frame_extractor.minio]")
	lines = append(lines, fmt.Sprintf("endpoint = '%s'", s.cfg.MinIO.Endpoint))
	lines = append(lines, fmt.Sprintf("bucket = '%s'", s.cfg.MinIO.Bucket))
	lines = append(lines, fmt.Sprintf("access_key = '%s'", s.cfg.MinIO.AccessKey))
	lines = append(lines, fmt.Sprintf("secret_key = '%s'", s.cfg.MinIO.SecretKey))
	lines = append(lines, fmt.Sprintf("use_ssl = %t", s.cfg.MinIO.UseSSL))
	lines = append(lines, fmt.Sprintf("base_path = '%s'", s.cfg.MinIO.BasePath))
	return lines
}

func (s *Service) buildTaskLines() []string {
	var lines []string
	lines = append(lines, "")
	for _, t := range s.cfg.Tasks {
		lines = append(lines, "[[frame_extractor.tasks]]")
		lines = append(lines, fmt.Sprintf("id = '%s'", t.ID))
		lines = append(lines, fmt.Sprintf("rtsp_url = '%s'", t.RtspURL))
		lines = append(lines, fmt.Sprintf("interval_ms = %d", t.IntervalMs))
		lines = append(lines, fmt.Sprintf("output_path = '%s'", t.OutputPath))
		lines = append(lines, fmt.Sprintf("enabled = %t", t.Enabled))
	}
	return lines
}

