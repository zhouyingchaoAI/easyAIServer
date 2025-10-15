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
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key==='service_id'">
            <a-tag color="blue">{{ record.service_id }}</a-tag>
          </template>
          <template v-else-if="column.key==='task_types'">
            <a-space>
              <a-tag v-for="type in record.task_types" :key="type" color="purple">
                {{ type }}
              </a-tag>
            </a-space>
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
  { title: '服务ID', key: 'service_id', width: 200 },
  { title: '服务名称', key: 'name', width: 200 },
  { title: '支持的任务类型', key: 'task_types', width: 300 },
  { title: '推理端点', key: 'endpoint', ellipsis: true },
  { title: '版本', key: 'version', width: 100 },
  { title: '状态', key: 'status', width: 120 },
  { title: '注册时间', key: 'register_at', width: 180 },
  { title: '最后心跳', key: 'last_heartbeat', width: 180 },
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

