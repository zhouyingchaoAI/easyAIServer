<template>
  <div class="p-4 gallery-container">
    <!-- 头部选择任务 -->
    <a-card :bordered="false" class="mb-4">
      <template #title>
        <span class="card-title">
          <PictureOutlined class="title-icon" />
          抽帧结果查看
        </span>
      </template>
      <a-row :gutter="16">
        <a-col :xs="24" :sm="16" :md="12">
          <a-select 
            v-model:value="selectedTaskId" 
            size="large" 
            placeholder="选择任务查看快照" 
            style="width: 100%"
            @change="onTaskChange"
          >
            <a-select-option v-for="task in tasks" :key="task.id" :value="task.id">
              <VideoCameraOutlined /> {{ task.id }} - {{ task.rtsp_url }}
            </a-select-option>
          </a-select>
        </a-col>
        <a-col :xs="24" :sm="8" :md="6">
          <a-button size="large" @click="fetchSnapshots" :loading="loading">
            <template #icon><ReloadOutlined /></template>
            刷新
          </a-button>
        </a-col>
      </a-row>
      
      <a-row v-if="selectedTaskId" class="mt-4">
        <a-col :span="24">
          <a-statistic-group>
            <a-statistic title="总快照数" :value="snapshots.length" />
            <a-statistic title="总大小" :value="totalSize" suffix="MB" :precision="2" />
            <a-statistic 
              title="最新快照" 
              :value="snapshots.length > 0 ? snapshots[0].filename : '-'" 
            />
          </a-statistic-group>
        </a-col>
      </a-row>
    </a-card>

    <!-- 图片展示区域 -->
    <a-card v-if="selectedTaskId" :bordered="false">
      <template #title>
        <span class="card-title">
          <FileImageOutlined class="title-icon" />
          快照列表 ({{ snapshots.length }})
        </span>
      </template>
      <template #extra>
        <a-space>
          <a-radio-group v-model:value="viewMode" button-style="solid" size="large">
            <a-radio-button value="grid">
              <AppstoreOutlined /> 网格
            </a-radio-button>
            <a-radio-button value="list">
              <UnorderedListOutlined /> 列表
            </a-radio-button>
          </a-radio-group>
        </a-space>
      </template>

      <a-spin :spinning="loading">
        <!-- 网格视图 -->
        <div v-if="viewMode === 'grid'" class="image-grid">
          <div 
            v-for="snap in snapshots" 
            :key="snap.path" 
            class="image-card"
          >
            <div class="image-wrapper" @click="() => onPreview(snap)">
              <img :src="snap.url" :alt="snap.filename" loading="lazy" />
              <div class="image-overlay">
                <EyeOutlined class="preview-icon" />
              </div>
            </div>
            <div class="image-info">
              <div class="filename" :title="snap.filename">{{ snap.filename }}</div>
              <div class="meta">
                <span><ClockCircleOutlined /> {{ formatTime(snap.mod_time) }}</span>
                <span><FileOutlined /> {{ formatSize(snap.size) }}</span>
              </div>
              <div class="actions">
                <a-button type="primary" size="small" @click="() => onPreview(snap)">
                  <template #icon><EyeOutlined /></template>
                  预览
                </a-button>
                <a-popconfirm title="确认删除?" @confirm="() => onDeleteSnap(snap)">
                  <a-button danger size="small">
                    <template #icon><DeleteOutlined /></template>
                    删除
                  </a-button>
                </a-popconfirm>
              </div>
            </div>
          </div>
        </div>

        <!-- 列表视图 -->
        <a-table 
          v-else
          :data-source="snapshots" 
          :columns="listColumns"
          row-key="path"
          :pagination="{ pageSize: 20, showTotal: (total) => `共 ${total} 条` }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key==='preview'">
              <img :src="record.url" class="thumbnail" @click="() => onPreview(record)" />
            </template>
            <template v-else-if="column.key==='filename'">
              <a-tooltip :title="record.path">
                {{ record.filename }}
              </a-tooltip>
            </template>
            <template v-else-if="column.key==='size'">
              {{ formatSize(record.size) }}
            </template>
            <template v-else-if="column.key==='mod_time'">
              {{ formatTime(record.mod_time) }}
            </template>
            <template v-else-if="column.key==='action'">
              <a-space>
                <a-button type="link" @click="() => onPreview(record)">
                  <template #icon><EyeOutlined /></template>
                  预览
                </a-button>
                <a-popconfirm title="确认删除?" @confirm="() => onDeleteSnap(record)">
                  <a-button type="link" danger>
                    <template #icon><DeleteOutlined /></template>
                    删除
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-spin>

      <!-- 空状态 -->
      <a-empty v-if="!loading && snapshots.length === 0" description="暂无快照数据" />
    </a-card>

    <!-- 图片预览弹窗 -->
    <a-modal 
      v-model:open="previewVisible" 
      :footer="null" 
      width="80%"
      :title="previewImage?.filename"
      centered
    >
      <div class="preview-content">
        <img :src="previewImage?.url" style="width: 100%;" />
      </div>
      <template #footer>
        <a-space>
          <a-button @click="previewVisible = false">关闭</a-button>
          <a-button type="primary" @click="downloadImage(previewImage)">
            <template #icon><DownloadOutlined /></template>
            下载
          </a-button>
          <a-popconfirm title="确认删除?" @confirm="() => { onDeleteSnap(previewImage); previewVisible = false; }">
            <a-button danger>
              <template #icon><DeleteOutlined /></template>
              删除
            </a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { 
  PictureOutlined, VideoCameraOutlined, ReloadOutlined, FileImageOutlined,
  AppstoreOutlined, UnorderedListOutlined, EyeOutlined, DeleteOutlined,
  ClockCircleOutlined, FileOutlined, DownloadOutlined
} from '@ant-design/icons-vue'
import { frameApi } from '@/api'

const route = useRoute()

const tasks = ref([])
const selectedTaskId = ref(null)
const snapshots = ref([])
const loading = ref(false)
const viewMode = ref('grid')
const previewVisible = ref(false)
const previewImage = ref(null)

const listColumns = [
  { title: '预览', key: 'preview', width: 100 },
  { title: '文件名', key: 'filename' },
  { title: '大小', key: 'size', width: 100 },
  { title: '时间', key: 'mod_time', width: 180 },
  { title: '操作', key: 'action', width: 150, fixed: 'right' },
]

const totalSize = computed(() => {
  const bytes = snapshots.value.reduce((sum, s) => sum + s.size, 0)
  return bytes / (1024 * 1024)
})

const fetchTasks = async () => {
  try {
    const { data } = await frameApi.listTasks()
    tasks.value = data?.items || []
    
    // check if task query param exists
    const queryTask = route.query.task
    if (queryTask && tasks.value.find(t => t.id === queryTask)) {
      selectedTaskId.value = queryTask
      fetchSnapshots()
    } else if (tasks.value.length > 0 && !selectedTaskId.value) {
      selectedTaskId.value = tasks.value[0].id
      fetchSnapshots()
    }
  } catch (e) {
    console.error('fetch tasks failed', e)
  }
}

const onTaskChange = () => {
  fetchSnapshots()
}

const fetchSnapshots = async () => {
  if (!selectedTaskId.value) return
  loading.value = true
  try {
    const { data } = await frameApi.listSnapshots(selectedTaskId.value)
    snapshots.value = data?.items || []
  } catch (e) {
    message.error(e?.response?.data?.error || '获取快照列表失败')
  } finally {
    loading.value = false
  }
}

const onPreview = (snap) => {
  previewImage.value = snap
  previewVisible.value = true
}

const onDeleteSnap = async (snap) => {
  try {
    await frameApi.deleteSnapshot(snap.task_id, snap.path)
    message.success('删除成功')
    await fetchSnapshots()
  } catch (e) {
    message.error(e?.response?.data?.error || '删除失败')
  }
}

const downloadImage = (snap) => {
  if (!snap) return
  const a = document.createElement('a')
  a.href = snap.url
  a.download = snap.filename
  a.click()
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const formatSize = (bytes) => {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

onMounted(fetchTasks)
</script>

<style scoped>
.gallery-container {
  background: #f0f2f5;
  min-height: calc(100vh - 64px);
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

.mt-4 {
  margin-top: 16px;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  margin-top: 16px;
}

.image-card {
  background: white;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 1px 6px -1px rgba(0, 0, 0, 0.02);
  transition: all 0.3s;
}

.image-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transform: translateY(-2px);
}

.image-wrapper {
  position: relative;
  width: 100%;
  padding-top: 75%; /* 4:3 aspect ratio */
  background: #f0f0f0;
  cursor: pointer;
  overflow: hidden;
}

.image-wrapper img {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s;
}

.image-wrapper:hover img {
  transform: scale(1.05);
}

.image-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.3s;
}

.image-wrapper:hover .image-overlay {
  opacity: 1;
}

.preview-icon {
  font-size: 48px;
  color: white;
}

.image-info {
  padding: 12px;
}

.filename {
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #8c8c8c;
  margin-bottom: 12px;
}

.meta span {
  display: flex;
  align-items: center;
  gap: 4px;
}

.actions {
  display: flex;
  gap: 8px;
}

.actions button {
  flex: 1;
}

.thumbnail {
  width: 60px;
  height: 45px;
  object-fit: cover;
  border-radius: 4px;
  cursor: pointer;
  transition: transform 0.2s;
}

.thumbnail:hover {
  transform: scale(1.1);
}

.preview-content {
  text-align: center;
  max-height: 70vh;
  overflow: auto;
}

:deep(.ant-statistic-group) {
  display: flex;
  gap: 48px;
}

:deep(.ant-card) {
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 1px 6px -1px rgba(0, 0, 0, 0.02);
  border-radius: 8px;
}
</style>

