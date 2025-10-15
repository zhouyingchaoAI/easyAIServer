package frameextractor

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log/slog"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "easydarwin/internal/conf"
    "easydarwin/utils/pkg/system"
)

// runOnceSnapshot pulls a single frame from RTSP and returns JPEG bytes.
func runOnceSnapshot(ctx context.Context, rtspURL string) ([]byte, error) {
    // ffmpeg -rtsp_transport tcp -stimeout 5000000 -i <url> -frames:v 1 -f mjpeg pipe:1
    ff := getFFmpegPath()
    args := []string{"-y", "-hide_banner", "-loglevel", "error", "-rtsp_transport", "tcp", "-stimeout", "5000000", "-i", rtspURL, "-frames:v", "1", "-f", "mjpeg", "pipe:1"}

    cmd := exec.CommandContext(ctx, ff, args...)
    var out bytes.Buffer
    cmd.Stdout = &out
    var errOut bytes.Buffer
    cmd.Stderr = &errOut
    if err := cmd.Run(); err != nil {
        if e := errOut.String(); e != "" {
            return nil, fmt.Errorf(e)
        }
        return nil, err
    }
    if out.Len() == 0 {
        if e := errOut.String(); e != "" {
            return nil, fmt.Errorf(e)
        }
        return nil, fmt.Errorf("empty snapshot output")
    }
    return out.Bytes(), nil
}

func getFFmpegPath() string {
    if runtime.GOOS == "windows" {
        return filepath.Join(system.GetCWD(), "ffmpeg.exe")
    }
    if runtime.GOOS == "darwin" {
        return "ffmpeg"
    }
    return filepath.Join(system.GetCWD(), "ffmpeg")
}

func (s *Service) runLocalSinkLoop(task conf.FrameExtractTask) {
    defer s.wg.Done()

    // ensure output directory exists
    baseDir := s.cfg.OutputDir
    if baseDir == "" {
        baseDir = filepath.Join(system.GetCWD(), "snapshots")
    }
    dir := filepath.Join(baseDir, task.OutputPath)
    _ = os.MkdirAll(dir, 0o755)

    minBackoff := 1 * time.Second
    maxBackoff := 30 * time.Second
    backoff := minBackoff

    for {
        select {
        case <-s.stop:
            return
        default:
        }

        // build and start continuous ffmpeg snapshotter
        args := buildContinuousArgs(task.RtspURL, dir, getIntervalMs(task, s.cfg))
        ff := getFFmpegPath()
        cmd := exec.Command(ff, args...)
        var stderr bytes.Buffer
        cmd.Stderr = &stderr

        if err := cmd.Start(); err != nil {
            s.log.Error("start ffmpeg failed", slog.String("task", task.ID), slog.String("err", err.Error()))
            // wait backoff then retry
            t := time.NewTimer(backoff)
            select {
            case <-s.stop:
                t.Stop()
                return
            case <-t.C:
            }
            backoff = nextBackoff(backoff, maxBackoff)
            continue
        }

        // wait process or stop signal
        procDone := make(chan error, 1)
        go func() { procDone <- cmd.Wait() }()

        select {
        case <-s.stop:
            _ = cmd.Process.Kill()
            <-procDone
            return
        case err := <-procDone:
            if err != nil {
                s.log.Warn("ffmpeg exited", slog.String("task", task.ID), slog.String("err", err.Error()), slog.String("stderr", truncate(stderr.String(), 512)))
            } else {
                s.log.Warn("ffmpeg exited normally", slog.String("task", task.ID))
            }
            // restart with backoff
            t := time.NewTimer(backoff)
            select {
            case <-s.stop:
                t.Stop()
                return
            case <-t.C:
            }
            backoff = nextBackoff(backoff, maxBackoff)
        }
    }
}

// ctx-bound versions used by runtime manager
func (s *Service) runLocalSinkLoopCtx(task conf.FrameExtractTask, stop <-chan struct{}) {
    defer s.wg.Done()

    baseDir := s.cfg.OutputDir
    if baseDir == "" {
        baseDir = filepath.Join(system.GetCWD(), "snapshots")
    }
    dir := filepath.Join(baseDir, task.OutputPath)
    _ = os.MkdirAll(dir, 0o755)

    minBackoff := 1 * time.Second
    maxBackoff := 30 * time.Second
    backoff := minBackoff

    for {
        select {
        case <-s.stop:
            return
        case <-stop:
            return
        default:
        }

        args := buildContinuousArgs(task.RtspURL, dir, getIntervalMs(task, s.cfg))
        ff := getFFmpegPath()
        cmd := exec.Command(ff, args...)
        var stderr bytes.Buffer
        cmd.Stderr = &stderr
        
        s.log.Info("starting ffmpeg", 
            slog.String("task", task.ID), 
            slog.String("output_dir", dir),
            slog.String("ffmpeg", ff),
            slog.String("cmd", strings.Join(args, " ")))

        if err := cmd.Start(); err != nil {
            s.log.Error("start ffmpeg failed", slog.String("task", task.ID), slog.String("err", err.Error()))
            t := time.NewTimer(backoff)
            select {
            case <-s.stop:
                t.Stop()
                return
            case <-stop:
                t.Stop()
                return
            case <-t.C:
            }
            backoff = nextBackoff(backoff, maxBackoff)
            continue
        }

        procDone := make(chan error, 1)
        go func() { procDone <- cmd.Wait() }()
        select {
        case <-s.stop:
            _ = cmd.Process.Kill()
            <-procDone
            return
        case <-stop:
            _ = cmd.Process.Kill()
            <-procDone
            return
        case err := <-procDone:
            if err != nil {
                s.log.Warn("ffmpeg exited", slog.String("task", task.ID), slog.String("err", err.Error()), slog.String("stderr", truncate(stderr.String(), 512)))
            } else {
                s.log.Warn("ffmpeg exited normally", slog.String("task", task.ID))
            }
            t := time.NewTimer(backoff)
            select {
            case <-s.stop:
                t.Stop()
                return
            case <-stop:
                t.Stop()
                return
            case <-t.C:
            }
            backoff = nextBackoff(backoff, maxBackoff)
        }
    }
}

func (s *Service) snapshotAndSave(task conf.FrameExtractTask, dir string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    data, err := runOnceSnapshot(ctx, task.RtspURL)
    if err != nil {
        // reduce log noise for frequent failures
        if !strings.Contains(strings.ToLower(err.Error()), "empty") {
            s.log.Warn("snapshot failed", slog.String("task", task.ID), slog.String("err", err.Error()))
        }
        return err
    }
    // filename with time
    ts := time.Now().Format("20060102-150405.000")
    name := fmt.Sprintf("%s.jpg", ts)
    file := filepath.Join(dir, name)
    // atomic write
    tmp := file + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        s.log.Error("write tmp failed", slog.String("task", task.ID), slog.String("err", err.Error()))
        return err
    }
    if err := os.Rename(tmp, file); err != nil {
        // fallback copy
        _ = os.Remove(tmp)
        f, e := os.Create(file)
        if e != nil {
            s.log.Error("create file failed", slog.String("task", task.ID), slog.String("err", e.Error()))
            return e
        }
        _, _ = io.Copy(f, bytes.NewReader(data))
        _ = f.Close()
    }
    return nil
}

func nextBackoff(cur, max time.Duration) time.Duration {
    next := cur * 2
    if next > max {
        return max
    }
    return next
}

// runMinioSinkLoop is a placeholder that will upload to MinIO in a next step.
// For now, it behaves like local sink to keep feature usable and code building.

func buildContinuousArgs(rtspURL, dir string, intervalMs int) []string {
    // fps filter: one frame per interval seconds
    if intervalMs <= 0 {
        intervalMs = 1000
    }
    sec := float64(intervalMs) / 1000.0
    // use strftime to timestamp filenames, image2 muxer
    // note: -reset_timestamps 1 helps after reconnects, though optional here
    args := []string{
        "-y", "-hide_banner", "-loglevel", "error",
        "-rtsp_transport", "tcp",
        "-stimeout", "5000000",
        "-i", rtspURL,
        "-vf", fmt.Sprintf("fps=1/%.6f", sec),
        "-f", "image2",
        "-strftime", "1",
        filepath.Join(dir, "%Y%m%d-%H%M%S.jpg"),
    }
    return args
}

func truncate(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n]
}


