<template>
  <div class="p-4 minio-monitor-container">
    <a-card :bordered="false" class="monitor-card">
      <template #title>
        <span class="card-title">
          <DatabaseOutlined class="title-icon" />
          MinIO图片移动监控
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
        </a-space>
      </template>

      <!-- 概览统计卡片 -->
      <a-row :gutter="16" class="mb-4">
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="总移动次数"
              :value="stats.minio_move_total || 0"
              :value-style="{ color: '#1890ff' }"
            >
              <template #prefix>
                <SwapOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="成功次数"
              :value="stats.minio_move_success || 0"
              :value-style="{ color: '#52c41a' }"
            >
              <template #prefix>
                <CheckCircleOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="失败次数"
              :value="stats.minio_move_failed || 0"
              :value-style="getFailedColor()"
            >
              <template #prefix>
                <CloseCircleOutlined />
              </template>
            </a-statistic>
          </a-card>
        </a-col>
        <a-col :xs="24" :sm="12" :md="6">
          <a-card class="stat-card">
            <a-statistic
              title="成功率"
              :value="(stats.minio_move_success_rate * 100 || 0).toFixed(2)"
              :value-style="getSuccessRateColor()"
            >
              <template #prefix>
                <PercentageOutlined />
              </template>
              <template #suffix>
                <span style="font-size: 14px; color: #999;">%</span>
              </template>
            </a-statistic>
          </a-card>
        </a-col>
      </a-row>

      <!-- 性能指标卡片 -->
      <a-card title="性能指标" size="small" class="mb-4">
        <a-row :gutter="16">
          <a-col :xs="24" :sm="12" :md="8">
            <a-statistic
              title="平均响应时间"
              :value="stats.minio_move_avg_time_ms || 0"
              :precision="2"
              :value-style="getAvgTimeColor()"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">ms</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="12" :md="8">
            <a-statistic
              title="最大响应时间"
              :value="stats.minio_move_max_time_ms || 0"
              :value-style="getMaxTimeColor()"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">ms</span>
              </template>
            </a-statistic>
          </a-col>
          <a-col :xs="24" :sm="12" :md="8">
            <a-statistic
              title="并发限制"
              :value="concurrentLimit"
              :value-style="{ color: '#1890ff' }"
            >
              <template #suffix>
                <span style="font-size: 14px; color: #999;">个</span>
              </template>
            </a-statistic>
          </a-col>
        </a-row>
      </a-card>

      <!-- 性能趋势图表（可选） -->
      <a-card title="性能趋势" size="small" class="mb-4">
        <a-empty v-if="!hasData" description="暂无数据" />
        <div v-else class="performance-chart">
          <a-alert
            type="info"
            show-icon
            :message="getPerformanceMessage()"
            style="margin-bottom: 16px;"
          />
          <a-descriptions :column="2" bordered size="small">
            <a-descriptions-item label="总移动次数">
              {{ stats.minio_move_total || 0 }}
            </a-descriptions-item>
            <a-descriptions-item label="成功率">
              <a-tag :color="getSuccessRateTagColor()">
                {{ (stats.minio_move_success_rate * 100 || 0).toFixed(2) }}%
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="平均响应时间">
              <a-tag :color="getAvgTimeTagColor()">
                {{ (stats.minio_move_avg_time_ms || 0).toFixed(2) }}ms
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="最大响应时间">
              <a-tag :color="getMaxTimeTagColor()">
                {{ stats.minio_move_max_time_ms || 0 }}ms
              </a-tag>
            </a-descriptions-item>
          </a-descriptions>
        </div>
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
  DatabaseOutlined,
  ReloadOutlined,
  SwapOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  PercentageOutlined
} from '@ant-design/icons-vue'
import { aiAnalysisApi } from '@/api'

const loading = ref(false)
const autoRefresh = ref(true)
const refreshInterval = ref(3000)
const concurrentLimit = ref(50) // 从配置读取，这里先写死
let timer = null

const stats = ref({
  minio_move_total: 0,
  minio_move_success: 0,
  minio_move_failed: 0,
  minio_move_avg_time_ms: 0,
  minio_move_max_time_ms: 0,
  minio_move_success_rate: 0,
  updated_at: ''
})

const hasData = ref(false)

const fetchStats = async () => {
  loading.value = true
  try {
    const response = await aiAnalysisApi.getInferenceStats()
    if (response.data) {
      stats.value = {
        minio_move_total: response.data.minio_move_total || 0,
        minio_move_success: response.data.minio_move_success || 0,
        minio_move_failed: response.data.minio_move_failed || 0,
        minio_move_avg_time_ms: response.data.minio_move_avg_time_ms || 0,
        minio_move_max_time_ms: response.data.minio_move_max_time_ms || 0,
        minio_move_success_rate: response.data.minio_move_success_rate || 0,
        updated_at: response.data.updated_at || ''
      }
      hasData.value = stats.value.minio_move_total > 0
    }
  } catch (e) {
    console.error('fetch minio stats failed', e)
    message.error('获取MinIO监控数据失败')
  } finally {
    loading.value = false
  }
}

// 失败次数颜色
const getFailedColor = () => {
  const failed = stats.value.minio_move_failed || 0
  if (failed > 0) return { color: '#ff4d4f' }
  return { color: '#52c41a' }
}

// 成功率颜色
const getSuccessRateColor = () => {
  const rate = stats.value.minio_move_success_rate || 0
  if (rate >= 0.99) return { color: '#52c41a' }
  if (rate >= 0.95) return { color: '#faad14' }
  return { color: '#ff4d4f' }
}

// 平均响应时间颜色
const getAvgTimeColor = () => {
  const avgTime = stats.value.minio_move_avg_time_ms || 0
  if (avgTime < 200) return { color: '#52c41a' }
  if (avgTime < 500) return { color: '#faad14' }
  return { color: '#ff4d4f' }
}

// 最大响应时间颜色
const getMaxTimeColor = () => {
  const maxTime = stats.value.minio_move_max_time_ms || 0
  if (maxTime < 1000) return { color: '#52c41a' }
  if (maxTime < 2000) return { color: '#faad14' }
  return { color: '#ff4d4f' }
}

// 成功率标签颜色
const getSuccessRateTagColor = () => {
  const rate = stats.value.minio_move_success_rate || 0
  if (rate >= 0.99) return 'green'
  if (rate >= 0.95) return 'orange'
  return 'red'
}

// 平均响应时间标签颜色
const getAvgTimeTagColor = () => {
  const avgTime = stats.value.minio_move_avg_time_ms || 0
  if (avgTime < 200) return 'green'
  if (avgTime < 500) return 'orange'
  return 'red'
}

// 最大响应时间标签颜色
const getMaxTimeTagColor = () => {
  const maxTime = stats.value.minio_move_max_time_ms || 0
  if (maxTime < 1000) return 'green'
  if (maxTime < 2000) return 'orange'
  return 'red'
}

// 性能提示信息
const getPerformanceMessage = () => {
  const rate = stats.value.minio_move_success_rate || 0
  const avgTime = stats.value.minio_move_avg_time_ms || 0
  
  if (rate >= 0.99 && avgTime < 200) {
    return 'MinIO性能正常：成功率高，响应时间快'
  }
  if (rate < 0.95) {
    return '警告：MinIO操作成功率较低，建议检查MinIO服务器状态'
  }
  if (avgTime > 500) {
    return '警告：MinIO响应时间较长，建议优化MinIO配置或检查网络'
  }
  return 'MinIO性能良好'
}

// 格式化时间
const formatTime = (dateStr) => {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return dateStr
  }
}

// 自动刷新
watch([autoRefresh, refreshInterval], () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  
  if (autoRefresh.value) {
    timer = setInterval(() => {
      fetchStats()
    }, refreshInterval.value)
  }
}, { immediate: true })

onMounted(() => {
  fetchStats()
})

onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
})
</script>

<style scoped>
.minio-monitor-container {
  min-height: 100vh;
  background: #f0f2f5;
}

.monitor-card {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.card-title {
  display: flex;
  align-items: center;
  font-size: 18px;
  font-weight: 600;
}

.title-icon {
  margin-right: 8px;
  font-size: 20px;
  color: #1890ff;
}

.stat-card {
  text-align: center;
  transition: all 0.3s;
}

.stat-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transform: translateY(-2px);
}

.performance-chart {
  min-height: 200px;
}
</style>

