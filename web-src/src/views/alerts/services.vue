<template>
  <div class="p-4 services-container">
    <a-card :bordered="false" class="services-card">
      <template #title>
        <span class="card-title">
          <ApiOutlined class="title-icon" />
          注册的算法服务
        </span>
      </template>
      <template #extra>
        <a-button @click="fetchServices" size="small">
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
      </template>

      <a-table 
        :data-source="services" 
        :columns="columns" 
        :loading="loading"
        row-key="service_id" 
        :pagination="false"
        :scroll="{ x: 1400 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key==='service_id'">
            <a-tooltip :title="record.service_id">
              <a-tag color="blue" style="max-width: 160px; overflow: hidden; text-overflow: ellipsis;">
                {{ record.service_id }}
              </a-tag>
            </a-tooltip>
          </template>
          <template v-else-if="column.key==='task_types'">
            <div style="max-height: 100px; overflow-y: auto;">
              <a-space wrap size="small">
                <a-tag v-for="type in record.task_types" :key="type" color="purple" style="margin-bottom: 4px;">
                  {{ type }}
                </a-tag>
              </a-space>
            </div>
          </template>
          <template v-else-if="column.key==='endpoint'">
            <a-tooltip :title="record.endpoint">
              <span style="display: block; max-width: 230px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                {{ record.endpoint }}
              </span>
            </a-tooltip>
          </template>
          <template v-else-if="column.key==='status'">
            <a-badge :status="getServiceStatus(record)" :text="getServiceStatusText(record)" />
          </template>
          <template v-else-if="column.key==='register_at'">
            {{ formatTime(record.register_at) }}
          </template>
          <template v-else-if="column.key==='last_heartbeat'">
            {{ formatTime(record.last_heartbeat) }}
          </template>
          <template v-else-if="column.key==='call_count'">
            <a-tag color="cyan">{{ formatNumber(record.call_count || 0) }}</a-tag>
          </template>
          <template v-else-if="column.key==='last_inference_time_ms'">
            <a-tag v-if="record.last_inference_time_ms > 0" color="blue">
              {{ formatMs(record.last_inference_time_ms) }}
            </a-tag>
            <span v-else style="color: #999;">-</span>
          </template>
          <template v-else-if="column.key==='last_total_time_ms'">
            <a-tag v-if="record.last_total_time_ms > 0" color="purple">
              {{ formatMs(record.last_total_time_ms) }}
            </a-tag>
            <span v-else style="color: #999;">-</span>
          </template>
          <template v-else-if="column.key==='avg_inference_time_ms'">
            <a-tag v-if="record.avg_inference_time_ms > 0" :color="getPerformanceColor(record.avg_inference_time_ms)">
              {{ formatMs(record.avg_inference_time_ms) }}
            </a-tag>
            <span v-else style="color: #999;">-</span>
          </template>
        </template>
      </a-table>

      <a-alert
        v-if="services.length === 0 && !loading"
        message="暂无算法服务注册"
        description="算法服务需要主动调用API注册到EasyDarwin。参考文档：doc/AI_ANALYSIS.md"
        type="info"
        show-icon
        class="mt-4"
      />
    </a-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { ApiOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { alertApi } from '@/api'

const loading = ref(false)
const services = ref([])

const columns = [
  { title: '服务ID', key: 'service_id', width: 180, ellipsis: true },
  { title: '服务名称', key: 'name', width: 150 },
  { title: '支持的任务类型', key: 'task_types', width: 250 },
  { title: '推理端点', key: 'endpoint', width: 220, ellipsis: true },
  { title: '版本', key: 'version', width: 80 },
  { title: '状态', key: 'status', width: 100 },
  { title: '调用次数', key: 'call_count', width: 100, align: 'center' },
  { title: '推理时间', key: 'last_inference_time_ms', width: 100, align: 'center' },
  { title: '总耗时', key: 'last_total_time_ms', width: 100, align: 'center' },
  { title: '平均耗时', key: 'avg_inference_time_ms', width: 100, align: 'center' },
  { title: '最后心跳', key: 'last_heartbeat', width: 150 },
]

const fetchServices = async () => {
  loading.value = true
  try {
    const { data } = await alertApi.listServices()
    services.value = data?.services || []
  } catch (e) {
    console.error('fetch services failed', e)
    message.error('加载算法服务列表失败')
  } finally {
    loading.value = false
  }
}

const getServiceStatus = (record) => {
  const now = Math.floor(Date.now() / 1000)
  const lastHB = record.last_heartbeat || 0
  const diff = now - lastHB
  
  if (diff < 90) return 'success'
  if (diff < 180) return 'warning'
  return 'error'
}

const getServiceStatusText = (record) => {
  const now = Math.floor(Date.now() / 1000)
  const lastHB = record.last_heartbeat || 0
  const diff = now - lastHB
  
  if (diff < 90) return '正常'
  if (diff < 180) return '心跳超时'
  return '已失联'
}

const formatTime = (timestamp) => {
  if (!timestamp) return '-'
  const date = new Date(timestamp * 1000)
  return date.toLocaleString('zh-CN')
}

// 格式化毫秒数
const formatMs = (ms) => {
  if (!ms || ms === 0) return '-'
  return `${ms.toFixed(2)}ms`
}

// 格式化数字（添加千位分隔符）
const formatNumber = (num) => {
  if (!num) return '0'
  return num.toLocaleString('zh-CN')
}

// 根据性能获取颜色
const getPerformanceColor = (avgMs) => {
  if (avgMs < 50) return 'green'    // 快速
  if (avgMs < 100) return 'blue'    // 良好
  if (avgMs < 200) return 'orange'  // 一般
  return 'red'                      // 慢速
}

onMounted(() => {
  fetchServices()
  // 自动刷新
  setInterval(fetchServices, 30000)
})
</script>

<style scoped>
.services-container {
  background: #f0f2f5;
  min-height: calc(100vh - 64px);
}

.services-card {
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

.mt-4 {
  margin-top: 16px;
}

:deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  font-weight: 600;
}
</style>

