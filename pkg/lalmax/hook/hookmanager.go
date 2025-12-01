package hook

import (
	"strings"
	"sync"

	"github.com/q191201771/naza/pkg/nazalog"
)

type HookSessionMangaer struct {
	sessionMap sync.Map
}

var (
	manager *HookSessionMangaer
	once    sync.Once
)

func GetHookSessionManagerInstance() *HookSessionMangaer {
	once.Do(func() {
		manager = &HookSessionMangaer{}
	})

	return manager
}

func (m *HookSessionMangaer) SetHookSession(streamName string, session *HookSession) {
	nazalog.Info("SetHookSession, streamName:", streamName)
	m.sessionMap.Store(streamName, session)
}

func (m *HookSessionMangaer) RemoveHookSession(streamName string) {
	nazalog.Info("RemoveHookSession, streamName:", streamName)
	// s, ok := m.sessionMap.Load(streamName)
	// if ok {
	m.sessionMap.Delete(streamName)
	// }
}

func (m *HookSessionMangaer) GetHookSession(streamName string) (bool, *HookSession) {
	if s, ok := m.sessionMap.Load(streamName); ok {
		return true, s.(*HookSession)
	}

	if alt := alternateStreamName(streamName); alt != "" {
		if s, ok := m.sessionMap.Load(alt); ok {
			nazalog.Infof("fallback hook session, requested:%s actual:%s", streamName, alt)
			return true, s.(*HookSession)
		}
	}

	return false, nil
}

func alternateStreamName(name string) string {
	switch {
	case strings.HasPrefix(name, "stream_"):
		return "video_" + strings.TrimPrefix(name, "stream_")
	case strings.HasPrefix(name, "video_"):
		return "stream_" + strings.TrimPrefix(name, "video_")
	default:
		return ""
	}
}
