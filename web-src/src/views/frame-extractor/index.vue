<template>
  <div class="p-4 frame-extractor-container">
    <!-- 存储配置区域 -->
    <a-card :bordered="false" class="config-card mb-4">
      <template #title>
        <span class="card-title">
          <CloudServerOutlined class="title-icon" />
          存储配置
        </span>
      </template>
      <a-form :model="config" layout="vertical">
        <a-row :gutter="24">
          <a-col :xs="24" :sm="12" :md="8">
            <a-form-item label="存储类型">
              <a-select v-model:value="config.store" size="large">
                <a-select-option value="local">
                  <FolderOutlined /> 本地文件系统
                </a-select-option>
                <a-select-option value="minio">
                  <CloudUploadOutlined /> MinIO对象存储
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :xs="24" :sm="12" :md="8">
            <a-form-item label="默认抽帧间隔">
              <a-input-number 
                v-model:value="config.interval_ms" 
                :min="200" 
                :step="100" 
                size="large"
                style="width: 100%"
              >
                <template #addonAfter>毫秒</template>
              </a-input-number>
            </a-form-item>
          </a-col>
          <a-col :xs="24" :sm="12" :md="8">
            <a-form-item label="启用状态">
              <a-switch 
                v-model:checked="config.enable" 
                size="large"
                checked-children="已启用"
                un-checked-children="已禁用"
              />
            </a-form-item>
          </a-col>
        </a-row>

        <a-form-item label="本地输出目录" v-if="config.store === 'local'">
          <a-input v-model:value="config.output_dir" size="large" placeholder="./snapshots">
            <template #prefix><FolderOpenOutlined /></template>
          </a-input>
        </a-form-item>

        <template v-if="config.store === 'minio'">
          <a-divider orientation="left">
            <CloudServerOutlined /> MinIO 配置
          </a-divider>
          <a-alert
            message="提示"
            description="Bucket不存在时会自动创建，任务ID会作为子路径，删除任务时会同步删除MinIO对应路径下的所有文件"
            type="info"
            show-icon
            class="mb-4"
          />
          <a-row :gutter="24">
            <a-col :xs="24" :sm="12">
              <a-form-item label="Endpoint">
                <a-input v-model:value="config.minio.endpoint" size="large" placeholder="minio.example.com:9000">
                  <template #prefix><ApiOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12">
              <a-form-item label="Bucket">
                <a-input v-model:value="config.minio.bucket" size="large" placeholder="snapshots">
                  <template #prefix><InboxOutlined /></template>
                  <template #suffix>
                    <a-tooltip title="不存在时会自动创建">
                      <InfoCircleOutlined style="color: rgba(0,0,0,.45)" />
                    </a-tooltip>
                  </template>
                </a-input>
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="24">
            <a-col :xs="24" :sm="12">
              <a-form-item label="Access Key">
                <a-input v-model:value="config.minio.access_key" size="large" placeholder="access key">
                  <template #prefix><KeyOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12">
              <a-form-item label="Secret Key">
                <a-input-password v-model:value="config.minio.secret_key" size="large" placeholder="secret key">
                  <template #prefix><LockOutlined /></template>
                </a-input-password>
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="24">
            <a-col :xs="24" :sm="12">
              <a-form-item label="Base Path (可选)">
                <a-input v-model:value="config.minio.base_path" size="large" placeholder="camera-frames">
                  <template #prefix><FolderOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12">
              <a-form-item label="使用 SSL">
                <a-switch 
                  v-model:checked="config.minio.use_ssl" 
                  size="large"
                  checked-children="HTTPS"
                  un-checked-children="HTTP"
                />
              </a-form-item>
            </a-col>
          </a-row>
        </template>

        <a-form-item>
          <a-button type="primary" size="large" @click="onSaveConfig" :loading="configLoading">
            <template #icon><SaveOutlined /></template>
            保存配置
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <!-- 任务管理区域 -->
    <a-card :bordered="false" class="task-card">
      <template #title>
        <span class="card-title">
          <VideoCameraOutlined class="title-icon" />
          抽帧任务 ({{ items.length }})
        </span>
      </template>
      <template #extra>
        <a-space>
          <a-tag :color="config.store === 'minio' ? 'blue' : 'green'">
            {{ config.store === 'minio' ? 'MinIO存储' : '本地存储' }}
          </a-tag>
          <a-button type="primary" @click="goToGallery">
            <template #icon><PictureOutlined /></template>
            查看抽帧结果
          </a-button>
        </a-space>
      </template>

      <!-- 添加任务表单 -->
      <a-card class="add-task-card mb-4" :bordered="false">
        <a-form :model="form" layout="vertical">
          <a-row :gutter="16">
            <a-col :xs="24" :sm="12" :md="6">
              <a-form-item label="任务ID" :required="true">
                <a-input v-model:value="form.id" size="large" placeholder="如 cam1">
                  <template #prefix><TagOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="10">
              <a-form-item label="RTSP地址" :required="true">
                <a-input v-model:value="form.rtsp_url" size="large" placeholder="rtsp://user:pass@ip:554/...">
                  <template #prefix><LinkOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="4">
              <a-form-item label="间隔(ms)">
                <a-input-number v-model:value="form.interval_ms" :min="200" :step="100" size="large" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="4">
              <a-form-item label="输出路径">
                <a-input v-model:value="form.output_path" size="large" placeholder="cam1">
                  <template #prefix><FolderOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item>
            <a-button type="primary" size="large" @click="onAdd" :loading="loading">
              <template #icon><PlusOutlined /></template>
              添加任务
            </a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <!-- 任务列表 -->
      <a-table 
        :data-source="items" 
        :columns="columns" 
        row-key="id" 
        :pagination="{ pageSize: 10, showTotal: (total) => `共 ${total} 条` }"
        :scroll="{ x: 800 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key==='id'">
            <a-tag color="blue">{{ record.id }}</a-tag>
          </template>
          <template v-else-if="column.key==='rtsp_url'">
            <a-tooltip :title="record.rtsp_url">
              <span class="rtsp-url">{{ record.rtsp_url }}</span>
            </a-tooltip>
          </template>
          <template v-else-if="column.key==='interval_ms'">
            <a-tag color="green">{{ record.interval_ms }}ms</a-tag>
          </template>
          <template v-else-if="column.key==='output_path'">
            <span>
              <FolderOutlined /> {{ record.output_path }}
            </span>
          </template>
          <template v-else-if="column.key==='action'">
            <a-space>
              <a-tooltip title="查看快照">
                <a-button type="default" size="small" @click="() => goToTaskGallery(record.id)">
                  <template #icon><PictureOutlined /></template>
                </a-button>
              </a-tooltip>
              <a-tooltip title="编辑">
                <a-button type="primary" size="small" @click="() => onEdit(record)">
                  <template #icon><EditOutlined /></template>
                </a-button>
              </a-tooltip>
              <a-popconfirm 
                title="确认删除?" 
                ok-text="删除"
                cancel-text="取消"
                @confirm="() => onDelete(record.id)"
              >
                <template #description>
                  <div style="max-width: 250px;">
                    <a-alert 
                      v-if="config.store === 'minio'"
                      message="将同时删除MinIO中的所有文件"
                      type="warning"
                      show-icon
                      :banner="true"
                      style="margin-bottom: 8px;"
                    />
                    此操作不可恢复，确认删除任务 <strong>{{ record.id }}</strong> 吗？
                  </div>
                </template>
                <a-button danger size="small">
                  <template #icon><DeleteOutlined /></template>
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
  </template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { 
  InfoCircleOutlined, CloudServerOutlined, CloudUploadOutlined,
  FolderOutlined, FolderOpenOutlined, ApiOutlined, InboxOutlined,
  KeyOutlined, LockOutlined, SaveOutlined, VideoCameraOutlined,
  TagOutlined, LinkOutlined, PlusOutlined, EditOutlined, DeleteOutlined,
  PictureOutlined
} from '@ant-design/icons-vue'
import { frameApi } from '@/api'

const router = useRouter()

const config = ref({
  enable: true,
  interval_ms: 1000,
  output_dir: './snapshots',
  store: 'local',
  minio: {
    endpoint: '',
    bucket: '',
    access_key: '',
    secret_key: '',
    use_ssl: false,
    base_path: ''
  }
})

const form = ref({ id: '', rtsp_url: '', interval_ms: 1000, output_path: '' })
const loading = ref(false)
const configLoading = ref(false)
const items = ref([])

const columns = [
  { title: '任务ID', key: 'id', width: 120 },
  { title: 'RTSP地址', key: 'rtsp_url', ellipsis: true },
  { title: '间隔', key: 'interval_ms', width: 100 },
  { title: '输出路径', key: 'output_path', width: 150 },
  { title: '操作', key: 'action', width: 180, fixed: 'right' },
]

const fetchConfig = async () => {
  try {
    const { data } = await frameApi.getConfig()
    if (data) {
      config.value = { ...config.value, ...data }
      if (!config.value.minio) {
        config.value.minio = {
          endpoint: '',
          bucket: '',
          access_key: '',
          secret_key: '',
          use_ssl: false,
          base_path: ''
        }
      }
    }
  } catch (e) {
    console.error('fetch config failed', e)
  }
}

const onSaveConfig = async () => {
  if (config.value.store === 'minio') {
    if (!config.value.minio.endpoint || !config.value.minio.bucket) {
      message.error('请填写MinIO Endpoint和Bucket')
      return
    }
  }
  configLoading.value = true
  try {
    await frameApi.updateConfig(config.value)
    message.success('配置保存成功')
  } catch (e) {
    message.error(e?.response?.data?.error || '配置保存失败')
  } finally {
    configLoading.value = false
  }
}

const fetchList = async () => {
  const { data } = await frameApi.listTasks()
  items.value = data?.items || []
}

const onAdd = async () => {
  if (!form.value.id || !form.value.rtsp_url) {
    message.error('请填写任务ID与RTSP地址')
    return
  }
  loading.value = true
  try {
    await frameApi.addTask(form.value)
    message.success('任务添加成功' + (config.value.store === 'minio' ? '，MinIO路径已创建' : ''))
    form.value = { id: '', rtsp_url: '', interval_ms: 1000, output_path: '' }
    await fetchList()
  } catch (e) {
    message.error(e?.response?.data?.error || '添加失败')
  } finally {
    loading.value = false
  }
}

const onEdit = (record) => {
  form.value = { ...record }
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const onDelete = async (id) => {
  try {
    await frameApi.delTask(id)
    message.success('任务删除成功' + (config.value.store === 'minio' ? '，MinIO路径已清理' : ''))
    await fetchList()
  } catch (e) {
    message.error(e?.response?.data?.error || '删除失败')
  }
}

const goToGallery = () => {
  router.push('/frame-extractor/gallery')
}

const goToTaskGallery = (taskId) => {
  router.push({ path: '/frame-extractor/gallery', query: { task: taskId } })
}

onMounted(() => {
  fetchConfig()
  fetchList()
})
</script>

<style scoped>
.frame-extractor-container {
  background: #f0f2f5;
  min-height: calc(100vh - 64px);
}

.config-card, .task-card {
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 1px 6px -1px rgba(0, 0, 0, 0.02), 0 2px 4px rgba(0, 0, 0, 0.02);
  border-radius: 8px;
}

.mb-4 {
  margin-bottom: 16px;
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

.add-task-card {
  background: #fafafa;
  border: 1px dashed #d9d9d9;
  border-radius: 4px;
}

.rtsp-url {
  max-width: 300px;
  display: inline-block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.ant-form-item) {
  margin-bottom: 16px;
}

:deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  font-weight: 600;
}
</style>
