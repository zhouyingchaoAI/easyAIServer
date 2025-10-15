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
      <template #extra>
        <a-button @click="fetchConfig" size="small">
          <template #icon><ReloadOutlined /></template>
          重新加载
        </a-button>
      </template>
      <a-alert 
        v-if="configLoadSuccess"
        message="配置已从服务器加载"
        type="success"
        show-icon
        closable
        class="mb-3"
        @close="configLoadSuccess = false"
      />
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
            <a-col :xs="24" :sm="12" :md="5">
              <a-form-item label="任务ID" :required="true">
                <a-input v-model:value="form.id" size="large" placeholder="如 cam1">
                  <template #prefix><TagOutlined /></template>
                </a-input>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="5">
              <a-form-item label="任务类型" :required="true">
                <a-select v-model:value="form.task_type" size="large" placeholder="选择类型">
                  <a-select-option v-for="type in taskTypes" :key="type" :value="type">
                    {{ type }}
                  </a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="8">
              <a-form-item>
                <template #label>
                  <span>
                    RTSP地址
                    <a-tooltip title="从直播列表选择或手动输入">
                      <InfoCircleOutlined style="margin-left: 4px; color: #8c8c8c;" />
                    </a-tooltip>
                    <a-button 
                      type="link" 
                      size="small" 
                      @click="fetchLiveStreams"
                      style="margin-left: 8px;"
                    >
                      <template #icon><ReloadOutlined /></template>
                      刷新列表
                    </a-button>
                  </span>
                </template>
                <a-auto-complete 
                  v-model:value="form.rtsp_url" 
                  :options="rtspOptions"
                  size="large" 
                  placeholder="选择直播流或输入RTSP地址"
                  :filter-option="filterRtspOption"
                  @select="onStreamSelect"
                >
                  <template #prefix><LinkOutlined /></template>
                </a-auto-complete>
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="3">
              <a-form-item label="间隔(ms)">
                <a-input-number v-model:value="form.interval_ms" :min="200" :step="100" size="large" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12" :md="3">
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
        :scroll="{ x: 1200 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key==='id'">
            <a-tag color="blue">{{ record.id }}</a-tag>
          </template>
          <template v-else-if="column.key==='task_type'">
            <a-tag color="purple">{{ record.task_type || '未分类' }}</a-tag>
          </template>
          <template v-else-if="column.key==='status'">
            <a-tag :color="record.enabled ? 'green' : 'default'">
              {{ record.enabled ? '运行中' : '已停止' }}
            </a-tag>
          </template>
          <template v-else-if="column.key==='rtsp_url'">
            <a-tooltip :title="record.rtsp_url">
              <span class="rtsp-url">{{ record.rtsp_url }}</span>
            </a-tooltip>
          </template>
          <template v-else-if="column.key==='interval_ms'">
            <a-popover trigger="click" placement="bottom">
              <template #content>
                <div style="width: 200px;">
                  <a-input-number 
                    v-model:value="editingInterval[record.id]" 
                    :min="200" 
                    :step="100" 
                    style="width: 100%; margin-bottom: 8px;"
                    placeholder="新间隔(ms)"
                  />
                  <a-button 
                    type="primary" 
                    size="small" 
                    block 
                    @click="() => onUpdateInterval(record)"
                  >
                    确认修改
                  </a-button>
                </div>
              </template>
              <a-tag color="green" style="cursor: pointer;">
                {{ record.interval_ms }}ms <EditOutlined style="font-size: 10px;" />
              </a-tag>
            </a-popover>
          </template>
          <template v-else-if="column.key==='output_path'">
            <span>
              <FolderOutlined /> {{ record.output_path }}
            </span>
          </template>
          <template v-else-if="column.key==='action'">
            <a-space>
              <a-tooltip :title="record.enabled ? '停止抽帧' : '启动抽帧'">
                <a-button 
                  :type="record.enabled ? 'default' : 'primary'"
                  size="small" 
                  @click="() => record.enabled ? onStopTask(record.id) : onStartTask(record.id)"
                  :loading="taskActionLoading[record.id]"
                >
                  <template #icon>
                    <PauseCircleOutlined v-if="record.enabled" />
                    <PlayCircleOutlined v-else />
                  </template>
                </a-button>
              </a-tooltip>
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
  PictureOutlined, PlayCircleOutlined, PauseCircleOutlined, ReloadOutlined
} from '@ant-design/icons-vue'
import { frameApi, live } from '@/api'

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

const form = ref({ id: '', task_type: '', rtsp_url: '', interval_ms: 1000, output_path: '' })
const loading = ref(false)
const configLoading = ref(false)
const configLoadSuccess = ref(false)
const items = ref([])
const taskActionLoading = ref({})
const editingInterval = ref({})
const liveStreams = ref([])
const rtspOptions = ref([])
const taskTypes = ref([])

const columns = [
  { title: '任务ID', key: 'id', width: 120 },
  { title: '任务类型', key: 'task_type', width: 120 },
  { title: '状态', key: 'status', width: 100 },
  { title: 'RTSP地址', key: 'rtsp_url', ellipsis: true },
  { title: '间隔', key: 'interval_ms', width: 100 },
  { title: '操作', key: 'action', width: 280, fixed: 'right' },
]

const fetchConfig = async () => {
  try {
    const { data } = await frameApi.getConfig()
    console.log('fetched config from server:', data)
    if (data) {
      // ensure MinIO sub-config exists before merge
      if (!data.minio || typeof data.minio !== 'object') {
        data.minio = {
          endpoint: '',
          bucket: '',
          access_key: '',
          secret_key: '',
          use_ssl: false,
          base_path: ''
        }
      }
      // deep merge config
      config.value = {
        enable: data.enable !== undefined ? data.enable : config.value.enable,
        interval_ms: data.interval_ms || config.value.interval_ms,
        output_dir: data.output_dir || config.value.output_dir,
        store: data.store || config.value.store,
        minio: {
          endpoint: data.minio.endpoint || '',
          bucket: data.minio.bucket || '',
          access_key: data.minio.access_key || '',
          secret_key: data.minio.secret_key || '',
          use_ssl: data.minio.use_ssl || false,
          base_path: data.minio.base_path || ''
        }
      }
      console.log('config after merge:', config.value)
      configLoadSuccess.value = true
      setTimeout(() => { configLoadSuccess.value = false }, 3000)
    }
  } catch (e) {
    console.error('fetch config failed', e)
    message.error('加载配置失败')
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
    console.log('saving config:', config.value)
    await frameApi.updateConfig(config.value)
    message.success('配置保存成功，已持久化到config.toml')
    // reload config to verify
    await fetchConfig()
  } catch (e) {
    console.error('save config error:', e)
    message.error(e?.response?.data?.error || '配置保存失败')
  } finally {
    configLoading.value = false
  }
}

const fetchTaskTypes = async () => {
  try {
    const { data } = await frameApi.getTaskTypes()
    taskTypes.value = data?.task_types || []
    console.log('loaded task types:', taskTypes.value)
  } catch (e) {
    console.error('fetch task types failed', e)
  }
}

const fetchLiveStreams = async () => {
  try {
    const { data } = await live.getLiveList({})
    liveStreams.value = data?.items || []
    // build stream ID options for selection
    rtspOptions.value = liveStreams.value.map(stream => {
      const streamName = stream.name || `Stream ${stream.id}`
      return {
        value: String(stream.id), // store stream ID as value
        label: `${streamName} (ID: ${stream.id})`,
        streamId: stream.id,
        streamName: streamName
      }
    })
    console.log('loaded live streams:', rtspOptions.value.length)
  } catch (e) {
    console.error('fetch live streams failed', e)
  }
}

const filterRtspOption = (input, option) => {
  return option.label.toLowerCase().includes(input.toLowerCase())
}

const onStreamSelect = async (streamId, option) => {
  // if user manually typed an rtsp:// URL, keep it as is
  if (streamId && streamId.startsWith('rtsp://')) {
    return
  }
  
  // otherwise, treat it as stream ID and fetch play URL
  try {
    const id = parseInt(streamId)
    if (!id) {
      form.value.rtsp_url = '' // clear invalid input
      return
    }
    
    // show loading state
    const originalValue = form.value.rtsp_url
    form.value.rtsp_url = '获取播放地址中...'
    
    const { data } = await live.getPlayUrl(id)
    console.log('API response:', data)
    
    // API returns lowercase field names: rtsp, http_flv, etc.
    const rtspUrl = data?.info?.rtsp || data?.info?.RTSP
    if (rtspUrl) {
      form.value.rtsp_url = rtspUrl
      console.log('selected stream RTSP URL:', rtspUrl)
      message.success(`已选择: ${option.streamName}`)
    } else {
      form.value.rtsp_url = originalValue
      message.error('未获取到RTSP播放地址')
      console.warn('No RTSP URL found in response:', data)
    }
  } catch (e) {
    console.error('get play url failed', e)
    form.value.rtsp_url = ''
    message.error('获取播放地址失败: ' + (e?.response?.data?.error || e.message))
  }
}

const fetchList = async () => {
  const { data } = await frameApi.listTasks()
  items.value = data?.items || []
}

const onAdd = async () => {
  if (!form.value.id || !form.value.task_type || !form.value.rtsp_url) {
    message.error('请填写任务ID、任务类型与RTSP地址')
    return
  }
  loading.value = true
  try {
    await frameApi.addTask(form.value)
    message.success('任务添加成功' + (config.value.store === 'minio' ? '，MinIO路径已创建' : ''))
    form.value = { id: '', task_type: '', rtsp_url: '', interval_ms: 1000, output_path: '' }
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

const onStartTask = async (id) => {
  taskActionLoading.value[id] = true
  try {
    await frameApi.startTask(id)
    message.success('任务已启动')
    await fetchList()
  } catch (e) {
    message.error(e?.response?.data?.error || '启动失败')
  } finally {
    taskActionLoading.value[id] = false
  }
}

const onStopTask = async (id) => {
  taskActionLoading.value[id] = true
  try {
    await frameApi.stopTask(id)
    message.success('任务已停止')
    await fetchList()
  } catch (e) {
    message.error(e?.response?.data?.error || '停止失败')
  } finally {
    taskActionLoading.value[id] = false
  }
}

const onUpdateInterval = async (record) => {
  const newInterval = editingInterval.value[record.id] || record.interval_ms
  if (newInterval < 200) {
    message.error('间隔不能小于200ms')
    return
  }
  try {
    await frameApi.updateInterval(record.id, newInterval)
    message.success('间隔已更新' + (record.enabled ? '，任务已重启' : ''))
    await fetchList()
    delete editingInterval.value[record.id]
  } catch (e) {
    message.error(e?.response?.data?.error || '更新失败')
  }
}

onMounted(() => {
  fetchConfig()
  fetchList()
  fetchLiveStreams()
  fetchTaskTypes()
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

.mb-3 {
  margin-bottom: 12px;
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
