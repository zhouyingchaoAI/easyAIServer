<template>
  <div class="p-4 alerts-container">
    <a-card :bordered="false" class="alerts-card">
      <template #title>
        <span class="card-title">
          <BellOutlined class="title-icon" />
          智能分析告警
        </span>
      </template>
      <template #extra>
        <a-space>
          <a-button @click="fetchData" size="small">
            <template #icon><ReloadOutlined /></template>
            刷新
          </a-button>
          <a-button type="link" @click="goToServices">
            <template #icon><ApiOutlined /></template>
            算法服务
          </a-button>
        </a-space>
      </template>

      <!-- 筛选器 -->
      <a-row :gutter="16" class="mb-4">
        <a-col :xs="24" :sm="8" :md="6">
          <a-select 
            v-model:value="filter.task_type" 
            placeholder="任务类型" 
            allow-clear
            size="large"
            @change="fetchData"
          >
            <a-select-option value="">全部类型</a-select-option>
            <a-select-option v-for="type in taskTypes" :key="type" :value="type">
              {{ type }}
            </a-select-option>
          </a-select>
        </a-col>
        <a-col :xs="24" :sm="8" :md="6">
          <a-input 
            v-model:value="filter.task_id" 
            placeholder="任务ID" 
            allow-clear
            size="large"
            @pressEnter="fetchData"
          >
            <template #prefix><SearchOutlined /></template>
          </a-input>
        </a-col>
        <a-col :xs="24" :sm="8" :md="6">
          <a-button type="primary" size="large" @click="fetchData">
            <template #icon><SearchOutlined /></template>
            查询
          </a-button>
        </a-col>
      </a-row>

      <!-- 告警列表 -->
      <a-table 
        :data-source="alerts" 
        :columns="columns" 
        :loading="loading"
        row-key="id" 
        :pagination="pagination"
        @change="handleTableChange"
        :scroll="{ x: 1400 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key==='id'">
            <a-tag color="blue">#{{ record.id }}</a-tag>
          </template>
          <template v-else-if="column.key==='task_type'">
            <a-tag color="purple">{{ record.task_type }}</a-tag>
          </template>
          <template v-else-if="column.key==='task_id'">
            <a-tag color="cyan">{{ record.task_id }}</a-tag>
          </template>
          <template v-else-if="column.key==='algorithm_name'">
            <a-tag color="green">{{ record.algorithm_name }}</a-tag>
          </template>
          <template v-else-if="column.key==='confidence'">
            <a-progress 
              :percent="Math.round(record.confidence * 100)" 
              :strokeColor="record.confidence > 0.8 ? '#52c41a' : '#faad14'"
              size="small"
            />
          </template>
          <template v-else-if="column.key==='image_path'">
            <a-tooltip :title="record.image_path">
              <span class="image-path">{{ record.image_path }}</span>
            </a-tooltip>
          </template>
          <template v-else-if="column.key==='created_at'">
            {{ formatTime(record.created_at) }}
          </template>
          <template v-else-if="column.key==='inference_time_ms'">
            {{ record.inference_time_ms }}ms
          </template>
          <template v-else-if="column.key==='action'">
            <a-space>
              <a-button type="primary" size="small" @click="() => viewDetail(record)">
                <template #icon><EyeOutlined /></template>
                查看
              </a-button>
              <a-popconfirm 
                title="确认删除?" 
                @confirm="() => deleteAlert(record.id)"
              >
                <a-button danger size="small">
                  <template #icon><DeleteOutlined /></template>
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <!-- 告警详情模态框 -->
    <a-modal 
      v-model:open="detailVisible" 
      title="告警详情" 
      width="80%"
      :footer="null"
    >
      <div v-if="currentAlert">
        <a-row :gutter="24">
          <a-col :xs="24" :md="12">
            <a-card title="图片预览" size="small">
              <img :src="currentAlert.image_url" style="width: 100%;" />
            </a-card>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-card title="告警信息" size="small">
              <a-descriptions :column="1" bordered size="small">
                <a-descriptions-item label="告警ID">
                  #{{ currentAlert.id }}
                </a-descriptions-item>
                <a-descriptions-item label="任务ID">
                  <a-tag color="cyan">{{ currentAlert.task_id }}</a-tag>
                </a-descriptions-item>
                <a-descriptions-item label="任务类型">
                  <a-tag color="purple">{{ currentAlert.task_type }}</a-tag>
                </a-descriptions-item>
                <a-descriptions-item label="算法">
                  <div>
                    <a-tag color="green">{{ currentAlert.algorithm_name }}</a-tag>
                    <br />
                    <span class="text-gray-500 text-xs">{{ currentAlert.algorithm_id }}</span>
                  </div>
                </a-descriptions-item>
                <a-descriptions-item label="置信度">
                  <a-progress 
                    :percent="Math.round(currentAlert.confidence * 100)" 
                    :strokeColor="currentAlert.confidence > 0.8 ? '#52c41a' : '#faad14'"
                  />
                </a-descriptions-item>
                <a-descriptions-item label="推理时间">
                  {{ currentAlert.inference_time_ms }}ms
                </a-descriptions-item>
                <a-descriptions-item label="图片路径">
                  <code>{{ currentAlert.image_path }}</code>
                </a-descriptions-item>
                <a-descriptions-item label="告警时间">
                  {{ formatTime(currentAlert.created_at) }}
                </a-descriptions-item>
              </a-descriptions>
            </a-card>
            
            <a-card title="推理结果" size="small" class="mt-3">
              <pre class="result-json">{{ formatResult(currentAlert.result) }}</pre>
            </a-card>
          </a-col>
        </a-row>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { 
  BellOutlined, ReloadOutlined, ApiOutlined, SearchOutlined,
  EyeOutlined, DeleteOutlined
} from '@ant-design/icons-vue'
import { alertApi, frameApi } from '@/api'

const router = useRouter()

const loading = ref(false)
const alerts = ref([])
const taskTypes = ref([])
const detailVisible = ref(false)
const currentAlert = ref(null)

const filter = ref({
  task_id: '',
  task_type: '',
  page: 1,
  page_size: 20
})

const pagination = ref({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total) => `共 ${total} 条`
})

const columns = [
  { title: 'ID', key: 'id', width: 80 },
  { title: '任务类型', key: 'task_type', width: 120 },
  { title: '任务ID', key: 'task_id', width: 150 },
  { title: '算法', key: 'algorithm_name', width: 150 },
  { title: '置信度', key: 'confidence', width: 120 },
  { title: '图片路径', key: 'image_path', ellipsis: true },
  { title: '推理时间', key: 'inference_time_ms', width: 100 },
  { title: '告警时间', key: 'created_at', width: 180 },
  { title: '操作', key: 'action', width: 150, fixed: 'right' },
]

const fetchData = async () => {
  loading.value = true
  try {
    const { data } = await alertApi.listAlerts(filter.value)
    alerts.value = data?.items || []
    pagination.value.total = data?.total || 0
    pagination.value.current = filter.value.page
  } catch (e) {
    console.error('fetch alerts failed', e)
    message.error('加载告警列表失败')
  } finally {
    loading.value = false
  }
}

const fetchTaskTypes = async () => {
  try {
    const { data } = await frameApi.getTaskTypes()
    taskTypes.value = data?.task_types || []
  } catch (e) {
    console.error('fetch task types failed', e)
  }
}

const handleTableChange = (pag) => {
  filter.value.page = pag.current
  filter.value.page_size = pag.pageSize
  fetchData()
}

const viewDetail = (record) => {
  currentAlert.value = record
  detailVisible.value = true
}

const deleteAlert = async (id) => {
  try {
    await alertApi.deleteAlert(id)
    message.success('告警已删除')
    fetchData()
  } catch (e) {
    message.error('删除失败')
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

const formatResult = (resultStr) => {
  try {
    const obj = JSON.parse(resultStr)
    return JSON.stringify(obj, null, 2)
  } catch {
    return resultStr
  }
}

const goToServices = () => {
  router.push('/ai-services')
}

onMounted(() => {
  fetchData()
  fetchTaskTypes()
})
</script>

<style scoped>
.alerts-container {
  background: #f0f2f5;
  min-height: calc(100vh - 64px);
}

.alerts-card {
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 1px 6px -1px rgba(0, 0, 0, 0.02), 0 2px 4px rgba(0, 0, 0, 0.02);
  border-radius: 8px;
}

.mb-4 {
  margin-bottom: 16px;
}

.mt-3 {
  margin-top: 12px;
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

.image-path {
  max-width: 200px;
  display: inline-block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-json {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 4px;
  max-height: 400px;
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.6;
}

:deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  font-weight: 600;
}
</style>

