<template>
  <div class="p-4 monitor-container">
    <a-card :bordered="false" class="monitor-card">
      <template #title>
        <span class="card-title">
          <DashboardOutlined class="title-icon" />
          抽帧服务监控
        </span>
      </template>
      <template #extra>
        <a-space>
          <a-badge :status="autoRefresh ? 'processing' : 'default'" :text="autoRefresh ? '自动刷新中' : '已暂停'" />
          <a-switch v-model:checked="autoRefresh" checked-children="自动" un-checked-children="手动" />
          <a-select v-model:value="refreshInterval" style="width: 120px" :disabled="!autoRefresh">
            <a-select-option :value="1000">1秒</a-select-option>
            <a-select-option :value="3000">3秒</a-select-option>
            <a-select-option :value="5000">5秒</a-select-option>
            <a-select-option :value="10000">10秒</a-select-option>
          </a-select>
          <a-button @click="fetchStats" :loading="loading">
            <template #icon><ReloadOutlined /></template>
            刷新
          </a-button>
          <a-popconfirm
            title="确认清零统计数据？"
            ok-text="确定"
            cancel-text="取消"
            @confirm="resetStats"
          >
            <template #description>
              <div style="max-width: 300px;">
                <p>将清零以下统计数据：</p>
                <ul style="margin: 0; padding-left: 20px;">
                  <li>累计处理数</li>
                  <li>累计丢弃数</li>
                  <li>丢弃率</li>
                  <li>推理时间统计</li>
                </ul>
                <p style="color: #ff4d4f; margin-top: 8px;">注意：此操作不可恢复！</p>
              </div>
            </template>
            <a-button danger>
              <template #icon><ClearOutlined /></template>
              清零统计
            </a-button>
          </a-popconfirm>
        </a-space>
      </template>

      <!-- 概览统计卡片 -->
      <a-row :gutter="16" class="mb-4">
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="总任务数"
              :value="stats.total_tasks"
              :value-style="{ color: '#1890ff' }"
            >
              <template #prefix>
                <FileTextOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="运行中"
              :value="stats.running_tasks"
              :value-style="{ color: '#52c41a' }"
            >
              <template #prefix>
                <PlayCircleOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="已停止"
              :value="stats.stopped_tasks"
              :value-style="{ color: '#8c8c8c' }"
            >
              <template #prefix>
                <PauseCircleOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="待配置"
              :value="stats.pending_tasks"
              :value-style="{ color: '#faad14' }"
            >
              <template #prefix>
                <ExclamationCircleOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
      </a-row>

      <!-- 系统信息 -->
      <a-card title="系统资源" size="small" class="mb-4">
        <a-row :gutter="16">
          <a-col :xs="24" :sm="8">
            <a-statistic
              title="Goroutines"
              :value="stats.system_info.goroutines"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">个</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8">
            <a-statistic
              title="内存使用"
              :value="stats.system_info.memory_usage_mb"
              :precision="2"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">MB</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8">
            <a-statistic
              title="CPU核心"
              :value="stats.system_info.cpu_cores"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">核</span>
              </template>
            </a-statistic>
          </a-col>
        </a-row>
      </a-card>

      <!-- AI推理队列监控 -->
      <a-card title="AI推理队列" size="small" class="mb-4">
        <a-row :gutter="16" class="mb-3">
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="队列大小"
              :value="inferenceStats.queue_size"
              :value-style="getQueueSizeColor()"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">/ {{ inferenceStats.queue_max_size }}</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="队列使用率"
              :value="(inferenceStats.queue_utilization * 100).toFixed(1)"
              :value-style="getUtilizationColor()"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">%</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="图片丢弃率"
              :value="(inferenceStats.drop_rate * 100).toFixed(2)"
              :value-style="getDropRateColor()"
            >
              <template #prefix>
                <WarningOutlined v-if="inferenceStats.drop_rate > 0.1" />
              </template>
              <template #suffix>
                <span style="font-size: 14px; color: #999;">%</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="平均推理时间"
              :value="inferenceStats.avg_inference_ms"
              :precision="0"
              :value-style="getInferenceTimeColor()"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">ms</span>
              </template>
            </a-statistic>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="累计处理"
              :value="inferenceStats.processed_total"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">张</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="累计丢弃"
              :value="inferenceStats.dropped_total"
              :value-style="{ color: inferenceStats.dropped_total > 0 ? '#ff4d4f' : '#52c41a' }"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">张</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="推理成功"
              :value="inferenceStats.total_inferences"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">次</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="8" :md="6">
            <a-statistic
              title="推理失败"
              :value="inferenceStats.failed_inferences"
              :value-style="{ color: inferenceStats.failed_inferences > 0 ? '#ff4d4f' : '#52c41a' }"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">次</span>
              </template>
            </a-statistic>
          </a-col>
        </a-row>
        <!-- 告警提示 -->
        <div v-if="hasInferenceAlert()" class="mt-3">
          <a-alert :message="getInferenceAlertMessage()" :type="getInferenceAlertType()" show-icon />
        </div>
      </a-card>

      <!-- 任务列表详情 -->
      <a-card title="任务运行详情" size="small">
        <a-table 
          :data-source="stats.task_details" 
          :columns="columns" 
          :pagination="false"
          row-key="id"
          :scroll="{ x: 1200 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'id'">
              <a-tag color="blue">{{ record.id }}</a-tag>
            </template>
            <template v-else-if="column.key === 'task_type'">
              <a-tag color="purple">{{ record.task_type || '未分类' }}</a-tag>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-badge 
                :status="record.status === 'running' ? 'processing' : 'default'" 
                :text="record.status === 'running' ? '运行中' : '已停止'"
              />
            </template>
            <template v-else-if="column.key === 'config_status'">
              <a-tag :color="record.config_status === 'configured' ? 'green' : 'orange'">
                {{ record.config_status === 'configured' ? '已配置' : '待配置' }}
              </a-tag>
            </template>
            <template v-else-if="column.key === 'interval_ms'">
              {{ record.interval_ms }}ms
            </template>
            <template v-else-if="column.key === 'uptime'">
              {{ formatUptime(record.uptime) }}
            </template>
          </template>
        </a-table>
      </a-card>

      <!-- 更新时间 -->
      <div class="mt-3 text-right text-gray-500" style="font-size: 12px;">
        最后更新: {{ formatTime(stats.updated_at) }}
      </div>
    </a-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { message } from 'ant-design-vue'
import {
  DashboardOutlined,
  ReloadOutlined,
  FileTextOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ExclamationCircleOutlined,
  WarningOutlined,
  ClearOutlined
} from '@ant-design/icons-vue'
import { frameApi } from '@/api'
import request from '@/api/request'

const loading = ref(false)
const autoRefresh = ref(true)
const refreshInterval = ref(3000)
let timer = null

const stats = ref({
  total_tasks: 0,
  running_tasks: 0,
  stopped_tasks: 0,
  configured_tasks: 0,
  pending_tasks: 0,
  task_details: [],
  system_info: {
    goroutines: 0,
    memory_usage_mb: 0,
    cpu_cores: 0
  },
  updated_at: new Date()
})

const inferenceStats = ref({
  queue_size: 0,
  queue_max_size: 100,
  queue_utilization: 0,
  dropped_total: 0,
  processed_total: 0,
  drop_rate: 0,
  strategy: '',
  avg_inference_ms: 0,
  max_inference_ms: 0,
  total_inferences: 0,
  failed_inferences: 0,
  updated_at: ''
})

const columns = [
  { title: '任务ID', key: 'id', width: 150, fixed: 'left' },
  { title: '任务类型', key: 'task_type', width: 120 },
  { title: '状态', key: 'status', width: 100 },
  { title: '配置状态', key: 'config_status', width: 100 },
  { title: '抽帧间隔', key: 'interval_ms', width: 100 },
  { title: '输出路径', key: 'output_path', ellipsis: true },
  { title: '运行时长', key: 'uptime', width: 120 },
]

const fetchStats = async () => {
  loading.value = true
  try {
    // 获取抽帧服务统计
    const response = await frameApi.getStats()
    stats.value = response.data || response
    
    // 确保必要字段存在
    if (!stats.value.system_info) {
      stats.value.system_info = {
        goroutines: 0,
        memory_usage_mb: 0,
        cpu_cores: 0
      }
    }
    if (!stats.value.task_details) {
      stats.value.task_details = []
    }

    // 获取AI推理统计
    try {
      const inferenceResponse = await request({ url: '/ai_analysis/inference_stats', method: 'get' })
      if (inferenceResponse.data) {
        inferenceStats.value = inferenceResponse.data
      }
    } catch (inferenceError) {
      console.warn('inference stats not available:', inferenceError)
      // AI推理可能未启用，不影响主要功能
    }
  } catch (e) {
    console.error('fetch stats failed', e)
    message.error('获取监控数据失败')
  } finally {
    loading.value = false
  }
}

// 重置统计数据
const resetStats = async () => {
  try {
    // 重置AI推理统计
    const response = await request({ 
      url: '/ai_analysis/inference_stats/reset', 
      method: 'post' 
    })
    
    if (response.data && response.data.ok) {
      message.success('统计数据已清零')
      // 立即刷新数据
      await fetchStats()
    } else {
      message.warning('清零失败')
    }
  } catch (e) {
    console.error('reset stats failed', e)
    message.error('清零失败: ' + (e.response?.data?.error || e.message))
  }
}

// 队列大小颜色
const getQueueSizeColor = () => {
  const utilization = inferenceStats.value.queue_utilization
  if (utilization > 0.8) return { color: '#ff4d4f' }
  if (utilization > 0.5) return { color: '#faad14' }
  return { color: '#52c41a' }
}

// 使用率颜色
const getUtilizationColor = () => {
  const utilization = inferenceStats.value.queue_utilization
  if (utilization > 0.8) return { color: '#ff4d4f' }
  if (utilization > 0.5) return { color: '#faad14' }
  return { color: '#52c41a' }
}

// 丢弃率颜色
const getDropRateColor = () => {
  const dropRate = inferenceStats.value.drop_rate
  if (dropRate > 0.2) return { color: '#ff4d4f' }
  if (dropRate > 0.1) return { color: '#faad14' }
  if (dropRate > 0) return { color: '#faad14' }
  return { color: '#52c41a' }
}

// 推理时间颜色
const getInferenceTimeColor = () => {
  const avgTime = inferenceStats.value.avg_inference_ms
  if (avgTime > 3000) return { color: '#ff4d4f' }
  if (avgTime > 1000) return { color: '#faad14' }
  return { color: '#52c41a' }
}

// 检查是否有推理告警
const hasInferenceAlert = () => {
  const utilization = inferenceStats.value.queue_utilization
  const dropRate = inferenceStats.value.drop_rate
  return utilization > 0.8 || dropRate > 0.1
}

// 获取告警消息
const getInferenceAlertMessage = () => {
  const utilization = inferenceStats.value.queue_utilization
  const dropRate = inferenceStats.value.drop_rate
  
  if (utilization > 0.8 && dropRate > 0.1) {
    return `推理队列严重堆积（使用率${(utilization * 100).toFixed(1)}%），图片丢弃率过高（${(dropRate * 100).toFixed(2)}%），建议：1）增加AI算法服务实例 2）降低抽帧频率 3）检查算法服务性能`
  }
  if (utilization > 0.8) {
    return `推理队列堆积严重（使用率${(utilization * 100).toFixed(1)}%），建议增加AI算法服务实例或降低抽帧频率`
  }
  if (dropRate > 0.1) {
    return `图片丢弃率过高（${(dropRate * 100).toFixed(2)}%），推理能力不足，建议增加AI算法服务实例`
  }
  return ''
}

// 获取告警类型
const getInferenceAlertType = () => {
  const utilization = inferenceStats.value.queue_utilization
  const dropRate = inferenceStats.value.drop_rate
  
  if (utilization > 0.8 || dropRate > 0.2) return 'error'
  if (dropRate > 0.1) return 'warning'
  return 'warning'
}

const startAutoRefresh = () => {
  stopAutoRefresh()
  if (autoRefresh.value) {
    timer = setInterval(() => {
      fetchStats()
    }, refreshInterval.value)
  }
}

const stopAutoRefresh = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

const formatTime = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

const formatUptime = (seconds) => {
  if (!seconds || seconds === 0) return '-'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  
  if (hours > 0) {
    return `${hours}h ${minutes}m ${secs}s`
  } else if (minutes > 0) {
    return `${minutes}m ${secs}s`
  } else {
    return `${secs}s`
  }
}

watch([autoRefresh, refreshInterval], () => {
  startAutoRefresh()
})

onMounted(() => {
  fetchStats()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.monitor-container {
  background: #f0f2f5;
  min-height: calc(100vh - 64px);
}

.monitor-card {
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 1px 6px -1px rgba(0, 0, 0, 0.02), 0 2px 4px rgba(0, 0, 0, 0.02);
  border-radius: 8px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  display: flex;
  align-items: center;
}

.title-icon {
  font-size: 18px;
  margin-right: 8px;
  color: #1890ff;
}

.mb-4 {
  margin-bottom: 16px;
}

.mb-3 {
  margin-bottom: 12px;
}

.mt-3 {
  margin-top: 12px;
}

.text-right {
  text-align: right;
}

.text-gray-500 {
  color: #999;
}

.stat-card {
  border-radius: 6px;
  transition: all 0.3s;
}

.stat-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  transform: translateY(-2px);
}

:deep(.ant-statistic-title) {
  font-size: 14px;
  color: #666;
}

:deep(.ant-statistic-content-value) {
  font-size: 28px;
  font-weight: 600;
}

:deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  font-weight: 600;
}
</style>

