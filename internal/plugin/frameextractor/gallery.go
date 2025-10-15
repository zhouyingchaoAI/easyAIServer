package frameextractor

import (
	"context"
	"easydarwin/internal/conf"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// SnapshotInfo represents a single snapshot file
type SnapshotInfo struct {
	TaskID   string    `json:"task_id"`
	Filename string    `json:"filename"`
	Path     string    `json:"path"`      // relative path for access
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	URL      string    `json:"url"`       // preview URL
}

// ListSnapshots returns all snapshots for a task
func (s *Service) ListSnapshots(taskID string) ([]SnapshotInfo, error) {
	if s.cfg.Store == "minio" {
		return s.listMinioSnapshots(taskID)
	}
	return s.listLocalSnapshots(taskID)
}

// listLocalSnapshots lists snapshots from local filesystem
func (s *Service) listLocalSnapshots(taskID string) ([]SnapshotInfo, error) {
	// find task to get output_path
	var task *conf.FrameExtractTask
	for _, t := range s.cfg.Tasks {
		if t.ID == taskID {
			task = &t
			break
		}
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	
	baseDir := s.cfg.OutputDir
	if baseDir == "" {
		baseDir = "./snapshots"
	}
	dir := filepath.Join(baseDir, task.OutputPath)
	
	var results []SnapshotInfo
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".jpg") && !strings.HasSuffix(strings.ToLower(d.Name()), ".jpeg") {
			return nil
		}
		
		info, err := d.Info()
		if err != nil {
			return nil
		}
		
		relPath := strings.TrimPrefix(path, baseDir)
		relPath = strings.TrimPrefix(relPath, "/")
		relPath = strings.TrimPrefix(relPath, "\\")
		
		results = append(results, SnapshotInfo{
			TaskID:   taskID,
			Filename: d.Name(),
			Path:     relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			URL:      "/snapshots/" + relPath,
		})
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// sort by mod time desc
	sort.Slice(results, func(i, j int) bool {
		return results[i].ModTime.After(results[j].ModTime)
	})
	
	return results, nil
}

// listMinioSnapshots lists snapshots from MinIO
func (s *Service) listMinioSnapshots(taskID string) ([]SnapshotInfo, error) {
	if s.minio == nil {
		return nil, fmt.Errorf("minio not initialized")
	}
	
	// find task to get output_path
	var task *conf.FrameExtractTask
	for _, t := range s.cfg.Tasks {
		if t.ID == taskID {
			task = &t
			break
		}
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	
	prefix := filepath.Join(s.minio.base, task.OutputPath) + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	objectCh := s.minio.client.ListObjects(ctx, s.minio.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	
	var results []SnapshotInfo
	for object := range objectCh {
		if object.Err != nil {
			continue
		}
		
		name := filepath.Base(object.Key)
		if !strings.HasSuffix(strings.ToLower(name), ".jpg") && !strings.HasSuffix(strings.ToLower(name), ".jpeg") {
			continue
		}
		
		// generate presigned URL for preview (valid for 1 hour)
		presignedURL, err := s.minio.client.PresignedGetObject(ctx, s.minio.bucket, object.Key, time.Hour, nil)
		if err != nil {
			s.log.Warn("failed to generate presigned URL", slog.String("key", object.Key), slog.String("err", err.Error()))
			continue
		}
		
		results = append(results, SnapshotInfo{
			TaskID:   taskID,
			Filename: name,
			Path:     object.Key,
			Size:     object.Size,
			ModTime:  object.LastModified,
			URL:      presignedURL.String(),
		})
	}
	
	// sort by mod time desc
	sort.Slice(results, func(i, j int) bool {
		return results[i].ModTime.After(results[j].ModTime)
	})
	
	return results, nil
}

// DeleteSnapshot deletes a single snapshot
func (s *Service) DeleteSnapshot(taskID, path string) error {
	if s.cfg.Store == "minio" {
		return s.deleteMinioSnapshot(path)
	}
	return s.deleteLocalSnapshot(path)
}

// deleteLocalSnapshot deletes from local filesystem
func (s *Service) deleteLocalSnapshot(relPath string) error {
	baseDir := s.cfg.OutputDir
	if baseDir == "" {
		baseDir = "./snapshots"
	}
	fullPath := filepath.Join(baseDir, relPath)
	return os.Remove(fullPath)
}

// deleteMinioSnapshot deletes from MinIO
func (s *Service) deleteMinioSnapshot(key string) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	return s.minio.client.RemoveObject(ctx, s.minio.bucket, key, minio.RemoveObjectOptions{})
}

