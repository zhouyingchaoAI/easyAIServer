package conf

import (
	"easydarwin/pkg/lalmax/conf"
	"fmt"
	"github.com/q191201771/lal/pkg/hls"
	"github.com/q191201771/lal/pkg/logic"
	"github.com/q191201771/lal/pkg/rtsp"
	"github.com/q191201771/naza/pkg/nazalog"
	"golang.org/x/mod/sumdb/storage"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CommonHttpAddrConfig struct {
	HttpListenAddr  string `json:"http_listen_addr"`
	HttpsListenAddr string `json:"https_listen_addr"`
	HttpsCertFile   string `json:"https_cert_file"`
	HttpsKeyFile    string `json:"https_key_file"`
}

type DefaultHttpConfig struct {
	HttpsEnable          bool `json:"https_enable"`
	CommonHttpAddrConfig `mapstructure:",squash"`
}

type GopCacheConfig struct {
	GopNum               int `json:"gop_cache_num"`
	SingleGopMaxFrameNum int `json:"single_gop_max_frame_num"`
}

type HlsConfig struct {
	CommonHttpServerConfig `mapstructure:",squash"`
	hls.MuxerConfig        `mapstructure:",squash"`

	UseMemoryAsDiskFlag bool                   `json:"use_memory_as_disk_flag"`
	DiskUseMmapFlag     bool                   `json:"disk_use_mmap_flag"`
	UseM3u8MemoryFlag   bool                   `json:"use_m3u8_memory_flag"`
	SubSessionTimeoutMs int                    `json:"sub_session_timeout_ms"`
	SubSessionHashKey   string                 `json:"sub_session_hash_key"`
	Fmp4                CommonHttpServerConfig `json:"fmp4"`
	// RecordConfig        HlsConfigRecord
}

type HttpApiConfig struct {
	CommonHttpServerConfig `mapstructure:",squash"`
}

type HttpflvConfig struct {
	CommonHttpServerConfig `mapstructure:",squash"`
}

type HttpFmp4Config struct {
	CommonHttpServerConfig `mapstructure:",squash"`
}

type HttptsConfig struct {
	CommonHttpServerConfig `mapstructure:",squash"`
}
type InSessionConfig struct {
	AddDummyAudioEnable      bool `json:"add_dummy_audio_enable"`
	AddDummyAudioWaitAudioMs int  `json:"add_dummy_audio_wait_audio_ms"`
}

type RelayPushConfig struct {
	Enable   bool     `json:"enable"`
	AddrList []string `json:"addr_list"`
}

type RoomConfig struct {
	Enable    bool   `json:"enable"`
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}
type RtcConfig struct {
	PubTimeoutSec          uint32 `json:"pub_timeout_sec"`
	CommonHttpServerConfig `mapstructure:",squash"`
	RTCConfig              `mapstructure:",squash"`
}
type RtmpConfig struct {
	Enable                  bool   `json:"enable"`
	Addr                    string `json:"addr"`
	RtmpsEnable             bool   `json:"rtmps_enable"`
	RtmpsAddr               string `json:"rtmps_addr"`
	RtmpOverQuicEnable      bool   `json:"rtmp_over_quic_enable"`
	RtmpOverQuicAddr        string `json:"rtmp_over_quic_addr"`
	RtmpsCertFile           string `json:"rtmps_cert_file"`
	RtmpsKeyFile            string `json:"rtmps_key_file"`
	RtmpOverKcpEnable       bool   `json:"rtmp_over_kcp_enable"`
	RtmpOverKcpAddr         string `json:"rtmp_over_kcp_addr"`
	RtmpOverKcpDataShards   int    `json:"rtmp_over_kcp_data_shards"`
	RtmpOverKcpParityShards int    `json:"rtmp_over_kcp_parity_shards"`

	MergeWriteSize int    `json:"merge_write_size"`
	PubTimeoutSec  uint32 `json:"pub_timeout_sec"`
	PullTimeoutSec uint32 `json:"pull_timeout_sec"`
}
type RtspConfig struct {
	Enable                bool   `json:"enable"`
	Addr                  string `json:"addr"`
	RtspsEnable           bool   `json:"rtsps_enable"`
	RtspsAddr             string `json:"rtsps_addr"`
	RtspsCertFile         string `json:"rtsps_cert_file"`
	RtspsKeyFile          string `json:"rtsps_key_file"`
	OutWaitKeyFrameFlag   bool   `json:"out_wait_key_frame_flag"`
	WsRtspEnable          bool   `json:"ws_rtsp_enable"`
	WsRtspAddr            string `json:"ws_rtsp_addr"`
	TimeoutSec            uint32 `json:"timeout_sec"`
	PubTimeoutSec         uint32 `json:"pub_timeout_sec"`
	PullTimeoutSec        uint32 `json:"pull_timeout_sec"`
	rtsp.ServerAuthConfig `mapstructure:",squash"`
}
type StaticRelayPullConfig struct {
	Enable bool   `json:"enable"`
	Addr   string `json:"addr"`
}

type Bootstrap struct {
	LanIP                 string                `mapstructure:"lanip"`
	Debug                 bool                  `toml:"-" json:"-"`
	BuildVersion          string                `toml:"-" json:"-"`
	DaemonAddr            string                `mapstructure:"daemonaddr"`
	Base                  Base                  `mapstructure:"base"` //基础配置
	Data                  Database              `mapstructure:"data"` // 数据
	BaseLog               BaseLog               `mapstructure:"baselog"` // 日志
	DefaultHttpConfig     DefaultHttpConfig     `mapstructure:"defaulthttpconfig"`
	GopCacheConfig        GopCacheConfig        `json:"gop_cache_config" mapstructure:"gopcacheconfig"`
	HlsConfig             HlsConfig             `json:"hls" mapstructure:"hlsconfig"`
	HttpApiConfig         HttpApiConfig         `json:"http_api" mapstructure:"httpapiconfig"`
	HttpflvConfig         HttpflvConfig         `json:"httpflv" mapstructure:"httpflvconfig"`
	HttpFmp4Config        HttpFmp4Config        `json:"httpfmp4" mapstructure:"httpfmp4config"`
	HttptsConfig          HttptsConfig          `json:"httpts" mapstructure:"httptsconfig"`
	InSessionConfig       InSessionConfig       `json:"in_session" mapstructure:"insessionconfig"`
	LogConfig             nazalog.Option        `json:"log" mapstructure:"logconfig"`
	RecordConfig          RecordConfig          `json:"record" mapstructure:"recordconfig"`
	RelayPushConfig       RelayPushConfig       `json:"relay_push" mapstructure:"relaypushconfig"`
	RoomConfig            RoomConfig            `json:"room" mapstructure:"roomconfig"`
	RtcConfig             RtcConfig             `json:"rtc" mapstructure:"rtcconfig"`
	RtmpConfig            RtmpConfig            `json:"rtmp" mapstructure:"rtmpconfig"`
	VodConfig             VodConfig             `json:"vod" mapstructure:"vodconfig"`
	RtspConfig            RtspConfig            `json:"rtsp" mapstructure:"rtspconfig"`
	SrtConfig             config.SrtConfig      `json:"srt" mapstructure:"srtconfig"`
	StaticRelayPullConfig StaticRelayPullConfig `json:"static_relay_pull" mapstructure:"staticrelaypull"`
	LalConfig             LalConfig             `json:"lal" mapstructure:"lalconfig"` // LalConfig对应的config.toml中则需要是 lalconfig 选项

	// FrameExtractor 插件配置
	FrameExtractor       FrameExtractorConfig   `json:"frame_extractor" mapstructure:"frame_extractor"`
	
	// AIAnalysis 智能分析插件配置
	AIAnalysis           AIAnalysisConfig       `json:"ai_analysis" mapstructure:"ai_analysis"`

	*config.Config
	LogicCfg *logic.Config
}

type Base struct {
	DisabledCaptcha *bool  `json:"disabled_captcha"`                            //是否禁用登录验证码
	Timeout         int64  `json:"timeout" comment:"请求超时时间" `                   // 请求超时时间
	JwtSecret       string `json:"jwt_secret" comment:"jwt 秘钥，空串时，每次启动程序将随机赋值"` // JWT密钥
}

// Database 结构体，包含 Dsn、MaxIdleConns、MaxOpenConns、ConnMaxLifetime 和 SlowThreshold 五个字段
type Database struct {
	Dsn             string // 数据源名称
	MaxIdleConns    int32  // 最大空闲连接数
	MaxOpenConns    int32  // 最大打开连接数
	ConnMaxLifetime int64  // 连接最大生命周期
	SlowThreshold   int64  // 慢查询阈值
}

// Log 结构体，包含 Dir、Level、MaxAge、RotationTime 和 RotationSize 五个字段
type BaseLog struct {
	Dir          string `json:"dir" comment:"日志存储目录，不能使用特殊符号"`
	Level        string `json:"level" comment:"记录级别 debug/info/warn/error"`
	MaxAge       int64  `json:"max_age" comment:"保留日志多久，超过时间自动删除"`
	RotationTime int64  `json:"rotation_time" comment:"多久时间，分割一个新的日志文件"`
	RotationSize int64  `json:"rotation_size" comment:"多大文件，分割一个新的日志文件(MB)"`
}

type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	x, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(x)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration().String()), nil
}

func (d *Duration) Duration() time.Duration {
	return time.Duration(*d)
}
func GetPortInt(p string) int {
	var s string
	if strings.Contains(p, ":") {
		s = strings.Trim(p, ":")
	} else {
		s = p
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		v = 0
	}
	return v
}
func GetAddrString(p int) string {
	s := fmt.Sprintf(":%d", p)
	return s
}

type CommonHttpServerConfig struct {
	Enable      bool   `json:"enable"`
	EnableHttps bool   `json:"enable_https"`
	UrlPattern  string `json:"url_pattern"`
}
type StreamConfig struct {
	Enable               bool `json:"enable"`
	GopNum               int  `json:"gop_cache_num"`
	SingleGopMaxFrameNum int  `json:"single_gop_max_frame_num"`
	HttpFlvConfig        struct {
		CommonHttpServerConfig
	} `json:"http_flv"`
	HttpFmp4Config struct {
		CommonHttpServerConfig
	} `json:"http_fmp4"`
	HttpTsConfig struct {
		CommonHttpServerConfig
	} `json:"http_ts"`
}
type RTCConfig struct {
	ICEHostNATToIPs []string `json:"iceHostNatToIps"` // rtc服务公网IP，未设置使用内网
	ICEUDPMuxPort   int      `json:"iceUdpMuxPort"`   // rtc udp mux port
	ICETCPMuxPort   int      `json:"iceTcpMuxPort"`   // rtc tcp mux port
	PubTimeoutSec   uint32   `json:"pub_timeout_sec"`
	CommonHttpServerConfig
}
type RTSPConfig struct {
	Enable              bool   `json:"enable"`
	Addr                int    `json:"addr"`
	RtspsEnable         bool   `json:"rtsps_enable"`
	RtspsAddr           int    `json:"rtsps_addr"`
	OutWaitKeyFrameFlag bool   `json:"out_wait_key_frame_flag"`
	WsRtspEnable        bool   `json:"ws_rtsp_enable"`
	WsRtspAddr          int    `json:"ws_rtsp_addr"`
	PubTimeoutSec       uint32 `json:"pub_timeout_sec"`
	PullTimeoutSec      uint32 `json:"pull_timeout_sec"`
	AuthEnable          bool   `json:"auth_enable"`
	AuthMethod          int    `json:"auth_method"`
	UserName            string `json:"username"`
	PassWord            string `json:"password"`
}
type LalConfig struct {
	HttpListenAddr  string `json:"httplistenaddr"`
	HttpsListenAddr string `json:"httpslistenaddr"`
	HttpsCertFile   string `json:"httpscertfile"`
	HttpsKeyFile    string `json:"httpskeyfile"`
}
type VodConfig struct {
	Dir           string `json:"dir"`
	SrcDir        string `json:"src_dir"`
	SysTranNumber uint   `json:"sys_tran_number"`
	//ProgressNotifyURL 点播：点播转码进度回调
	ProgressNotifyURL string `json:"progress_notify_url"`
	//HlsTime 点播：点播转码切片时间
	HlsTime int `json:"hls_time"`
	//OpenSquare 点播：是否开启分享视频广场
	OpenSquare bool `json:"open_square"`
	//OpenDefinition 点播：是否开启多清晰度转码
	OpenDefinition bool `json:"open_definition"`
	//DefaultDefinition 点播：播放默认播放清晰度 hd
	DefaultDefinition string `json:"default_definition"`
	//TransDefinition 点播：待转码的清晰度 值是数组字符串 如:  sd,hd,fhd
	TransDefinition string `json:"trans_definition"`
	//TransVideo 点播：是否重新编码视频 默认0
	TransVideo bool `json:"trans_video"`
	// 转码方式,两种方式 软解码 libx264 、硬解码 h264_nvenc 方式，默认 libx264
	TranWay string `json:"tran_way"`
	// h265 的转码方式,三种方式：软解码 libx264 、硬解码 h264_nvenc、不变 copy，默认 libx264
	TranHevcWay string `json:"tran_hevc_way"`
}

// FrameExtractorConfig 抽帧插件配置
type FrameExtractorConfig struct {
    Enable       bool   `json:"enable" mapstructure:"enable"`
    // 全局默认抽帧间隔（毫秒），可被任务级覆盖
    IntervalMs   int    `json:"interval_ms" mapstructure:"interval_ms"`
    // 本地存储根目录
    OutputDir    string `json:"output_dir" mapstructure:"output_dir"`
    // 存储类型：local|minio
    Store        string `json:"store" mapstructure:"store"`
    // 任务类型列表，用于智能分析分类
    TaskTypes    []string `json:"task_types" mapstructure:"task_types"`
    // 全局默认最大抽帧图片数量（0表示不限制），可被任务级覆盖
    MaxFrameCount int `json:"max_frame_count" mapstructure:"max_frame_count"`
    // MinIO 配置（仅当 store==minio 时生效）
    MinIO MinIOConfig `json:"minio" mapstructure:"minio"`
    // 任务清单（可选），未配置时仅启用模块等待 API 下发
    Tasks []FrameExtractTask `json:"tasks" mapstructure:"tasks"`
}

type MinIOConfig struct {
    Endpoint  string `json:"endpoint" mapstructure:"endpoint"`
    Bucket    string `json:"bucket" mapstructure:"bucket"`
    AccessKey string `json:"access_key" mapstructure:"access_key"`
    SecretKey string `json:"secret_key" mapstructure:"secret_key"`
    UseSSL    bool   `json:"use_ssl" mapstructure:"use_ssl"`
    BasePath  string `json:"base_path" mapstructure:"base_path"`
}

type FrameExtractTask struct {
    ID           string `json:"id" mapstructure:"id"`
    TaskType     string `json:"task_type" mapstructure:"task_type"` // 任务类型，用于智能分析
    RtspURL      string `json:"rtsp_url" mapstructure:"rtsp_url"`
    IntervalMs   int    `json:"interval_ms" mapstructure:"interval_ms"`
    OutputPath   string `json:"output_path" mapstructure:"output_path"`
    Enabled      bool   `json:"enabled" mapstructure:"enabled"` // task running state
    ConfigStatus string `json:"config_status" mapstructure:"config_status"` // 配置状态: "unconfigured" | "configured"
    PreviewImage string `json:"preview_image" mapstructure:"preview_image"` // 预览图片路径
    MaxFrameCount int   `json:"max_frame_count" mapstructure:"max_frame_count"` // 最大抽帧图片数量（0或未配置时使用全局配置）
    SaveAlertImage *bool `json:"save_alert_image" mapstructure:"save_alert_image"` // 是否保存告警图片（nil表示使用全局配置，true/false表示任务级配置）
}
type RecordConfig struct {
	EnableFlv            bool   `json:"enable_flv"`
	FlvOutPath           string `json:"flv_out_path"`
	EnableMpegts         bool   `json:"enable_mpegts"`
	MpegtsOutPath        string `json:"mpegts_out_path"`
	EnableFmp4           bool   `json:"enable_fmp4"`
	Fmp4OutPath          string `json:"fmp4_out_path"`
	RecordInterval       int    `json:"record_interval"`        // 固定间隔录制一个文件，单位秒
	EnableRecordInterval bool   `json:"enable_record_interval"` // 是否开启固定间隔录制
}
type RTMPConfig struct {
	Enable             bool   `json:"enable"`
	Addr               int    `json:"addr"`
	RtmpsEnable        bool   `json:"rtmps_enable"`
	RtmpsAddr          int    `json:"rtmps_addr"`
	RtmpOverQuicEnable bool   `json:"rtmp_over_quic_enable"`
	RtmpOverQuicAddr   int    `json:"rtmp_over_quic_addr"`
	MergeWriteSize     int    `json:"merge_write_size"`
	PubTimeoutSec      uint32 `json:"pub_timeout_sec"`
	PullTimeoutSec     uint32 `json:"pull_timeout_sec"`
}
type StreamLogConfig struct {
	AssertBehavior      nazalog.AssertBehavior `json:"assert_behavior"` // 断言失败时的行为
	Level               nazalog.Level          `json:"level"`
	Filename            string                 `json:"filename"`               // 输出日志文件名，如果为空，则不写日志文件。可包含路径，路径不存在时，将自动创建
	IsToStdout          bool                   `json:"is_to_stdout"`           // 是否以stdout输出到控制台 TODO(chef): 再增加一个stderr的配置
	IsRotateDaily       bool                   `json:"is_rotate_daily"`        // 日志按天翻转
	ShortFileFlag       bool                   `json:"short_file_flag"`        // 是否在每行日志尾部添加源码文件及行号的信息
	TimestampFlag       bool                   `json:"timestamp_flag"`         // 是否在每行日志首部添加时间戳的信息
	TimestampWithMsFlag bool                   `json:"timestamp_with_ms_flag"` // 时间戳是否精确到毫秒
	LevelFlag           bool                   `json:"level_flag"`
}
type GetConfigInput struct {
	BaseConfig struct {
		Base
		HttpsListenAddr int    `json:"https_listen_addr"`
		HttpListenAddr  int    `json:"http_listen_addr"`
		HttpsKeyFile    string `json:"https_key_file"`
		HttpsCertFile   string `json:"https_cert_file"`
	} `json:"base"`
	DataConfig      Database `json:"data"`
	BaseLogConfig   BaseLog  `json:"base_log"`
	StreamConfig    `json:"stream"`
	HlsConfig       `json:"hls"`
	RTCConfig       `json:"rtc"`
	RTMPConfig      `json:"rtmp"`
	RTSPConfig      `json:"rtsp"`
	RecordConfig    `json:"record"`
	StreamLogConfig `json:"stream_log"`
}

type EditBaseInput struct {
	HttpsListenAddr int `json:"https_listen_addr"`
	HttpListenAddr  int `json:"http_listen_addr"`
	Base
}
type EditStreamInput struct {
	StreamConfig
}
type EditDatabaseInput struct {
	Database
}
type EditHlsInput struct {
	HlsConfig
}
type EditRtcInput struct {
	RTCConfig
}
type EditRtmpInput struct {
	RTMPConfig
}
type EditRtspInput struct {
	RTSPConfig
}
type EditRecordInput struct {
	RecordConfig
}
type EditStreamLogInput struct {
	StreamLogConfig
}
type EditBaseLogInput struct {
	BaseLog
}
type EditStorageInput struct {
	storage.Storage
}
type ServerHTTPS struct {
	Port     int    `comment:"https 端口"`
	CertFile string `comment:"证书文件地址, 相对路径时, 其父目录为配置目录"`
	KeyFile  string `comment:"私钥文件地址, 相对路径时, 其父目录为配置目录"`
}

func (s *Bootstrap) Cert() string {

	if filepath.IsAbs(s.DefaultHttpConfig.HttpsCertFile) {
		return s.DefaultHttpConfig.HttpsCertFile
	}
	return filepath.Join(Getwd(), s.DefaultHttpConfig.HttpsCertFile)
}
func Getwd() string {
	dir, _ := os.Getwd()
	return dir
}

func (s *Bootstrap) Key() string {
	if filepath.IsAbs(s.DefaultHttpConfig.HttpsKeyFile) {
		return s.DefaultHttpConfig.HttpsKeyFile
	}
	return filepath.Join(Getwd(), s.DefaultHttpConfig.HttpsKeyFile)
}

// AIAnalysisConfig 智能分析插件配置
type AIAnalysisConfig struct {
	Enable              bool    `json:"enable" mapstructure:"enable"`
	ScanIntervalSec     float64 `json:"scan_interval_sec" mapstructure:"scan_interval_sec"` // 支持小数，如0.2秒
	MQType              string `json:"mq_type" mapstructure:"mq_type"` // kafka|rabbitmq
	MQAddress           string `json:"mq_address" mapstructure:"mq_address"`
	MQTopic             string `json:"mq_topic" mapstructure:"mq_topic"`
	HeartbeatTimeoutSec int    `json:"heartbeat_timeout_sec" mapstructure:"heartbeat_timeout_sec"`
	MaxConcurrentInfer  int    `json:"max_concurrent_infer" mapstructure:"max_concurrent_infer"`
	MaxQueueSize        int    `json:"max_queue_size" mapstructure:"max_queue_size"`           // 推理队列最大容量，默认: 100
	SaveOnlyWithDetection bool `json:"save_only_with_detection" mapstructure:"save_only_with_detection"` // 只保存有检测结果的告警
	
	// 存储路径配置
	AlertBasePath string `json:"alert_base_path" mapstructure:"alert_base_path"` // 告警图片存储路径前缀，默认: alerts/
	
	// 批量写入配置
	AlertBatchEnabled     bool `json:"alert_batch_enabled" mapstructure:"alert_batch_enabled"`           // 是否启用批量写入，默认: true
	AlertBatchSize        int  `json:"alert_batch_size" mapstructure:"alert_batch_size"`                 // 批量写入大小，达到此数量触发写入，默认: 100
	AlertBatchIntervalSec int  `json:"alert_batch_interval_sec" mapstructure:"alert_batch_interval_sec"` // 批量写入刷新间隔（秒），默认: 2
	
	// 图片移动配置
	AlertImageMoveConcurrent int `json:"alert_image_move_concurrent" mapstructure:"alert_image_move_concurrent"` // 图片移动最大并发数，默认: 50
}

// AlgorithmService 算法服务注册信息
type AlgorithmService struct {
	ServiceID             string   `json:"service_id"`               // 服务唯一ID
	Name                  string   `json:"name"`                     // 服务名称
	TaskTypes             []string `json:"task_types"`               // 支持的任务类型
	Endpoint              string   `json:"endpoint"`                 // 推理HTTP端点
	Version               string   `json:"version"`                  // 版本号
	RegisterAt            int64    `json:"register_at"`              // 注册时间戳
	LastHeartbeat         int64    `json:"last_heartbeat"`           // 最后心跳时间戳
	
	// 性能统计（由心跳更新）
	TotalRequests         int64   `json:"total_requests"`            // 累积推理次数
	AvgInferenceTimeMs    float64 `json:"avg_inference_time_ms"`     // 平均推理时间（毫秒）
	LastInferenceTimeMs   float64 `json:"last_inference_time_ms"`    // 最近一次推理时间（毫秒）
	LastTotalTimeMs       float64 `json:"last_total_time_ms"`        // 最近一次总耗时（毫秒）
}

// HeartbeatRequest 心跳请求（可选携带统计数据）
type HeartbeatRequest struct {
	TotalRequests       int64   `json:"total_requests"`            // 累积推理次数
	AvgInferenceTimeMs  float64 `json:"avg_inference_time_ms"`     // 平均推理时间（毫秒）
	LastInferenceTimeMs float64 `json:"last_inference_time_ms"`    // 最近一次推理时间（毫秒）
	LastTotalTimeMs     float64 `json:"last_total_time_ms"`        // 最近一次总耗时（毫秒）
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ImageURL      string                 `json:"image_url"`       // MinIO预签名URL
	TaskID        string                 `json:"task_id"`         // 任务ID
	TaskType      string                 `json:"task_type"`       // 任务类型
	ImagePath     string                 `json:"image_path"`      // MinIO对象路径
	AlgoConfig    map[string]interface{} `json:"algo_config"`     // 算法配置（可选）
	AlgoConfigURL string                 `json:"algo_config_url"` // 算法配置文件URL（可选）
}

// InferenceResponse 推理响应
type InferenceResponse struct {
	Success         bool        `json:"success"`
	Result          interface{} `json:"result"` // 推理结果（JSON）
	Confidence      float64     `json:"confidence"`
	InferenceTimeMs float64     `json:"inference_time_ms"` // 改为float64，兼容算法服务返回的浮点数
	Error           string      `json:"error,omitempty"`
}
