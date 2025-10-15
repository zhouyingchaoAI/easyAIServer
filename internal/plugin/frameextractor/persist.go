package frameextractor

import (
	"fmt"
	"os"
	"strings"
)

// saveConfigToFile writes current config and tasks back to config.toml
func (s *Service) saveConfigToFile(cfgPath string) error {
	if cfgPath == "" {
		return nil // skip if no path provided
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	
	var out []string
	inFrameExtractor := false
	sectionStart := -1
	sectionEnd := -1
	
	// find [frame_extractor] section boundaries
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "[frame_extractor]" {
			inFrameExtractor = true
			sectionStart = i
			continue
		}
		
		// detect end of frame_extractor section
		if inFrameExtractor && strings.HasPrefix(trimmed, "[") && 
		   !strings.HasPrefix(trimmed, "[[frame_extractor") && 
		   !strings.HasPrefix(trimmed, "[frame_extractor.") {
			sectionEnd = i
			break
		}
	}
	
	// rebuild content
	if sectionStart >= 0 {
		// add lines before section
		out = append(out, lines[:sectionStart]...)
		
		// add new section
		out = append(out, "[frame_extractor]")
		out = append(out, s.buildConfigLines()...)
		out = append(out, s.buildTaskLines()...)
		
		// add lines after section
		if sectionEnd >= 0 {
			out = append(out, lines[sectionEnd:]...)
		} else {
			// section goes to EOF
		}
	} else {
		// section not found, append to end
		out = append(out, lines...)
		out = append(out, "")
		out = append(out, "[frame_extractor]")
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
