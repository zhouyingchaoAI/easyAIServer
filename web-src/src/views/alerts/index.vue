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
          <a-badge :count="selectedRowKeys.length" :offset="[10, 0]">
            <a-button @click="fetchData" size="small">
              <template #icon><ReloadOutlined /></template>
              刷新
            </a-button>
          </a-badge>
          <a-button type="link" @click="goToServices">
            <template #icon><ApiOutlined /></template>
            算法服务
          </a-button>
        </a-space>
      </template>

      <!-- 筛选器 -->
      <a-row :gutter="16" class="mb-4">
        <a-col :xs="24" :sm="12" :md="6">
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
        <a-col :xs="24" :sm="12" :md="5">
          <a-select 
            v-model:value="filter.task_id" 
            placeholder="任务ID" 
            allow-clear
            show-search
            size="large"
            :filter-option="filterOption"
            @change="fetchData"
          >
            <a-select-option value="">全部任务</a-select-option>
            <a-select-option v-for="taskId in taskIds" :key="taskId" :value="taskId">
              {{ taskId }}
            </a-select-option>
          </a-select>
        </a-col>
        <a-col :xs="12" :sm="12" :md="4">
          <a-input-number 
            v-model:value="filter.min_detections" 
            placeholder="最少检测数" 
            :min="0"
            size="large"
            style="width: 100%"
          />
        </a-col>
        <a-col :xs="12" :sm="12" :md="4">
          <a-input-number 
            v-model:value="filter.max_detections" 
            placeholder="最多检测数" 
            :min="0"
            size="large"
            style="width: 100%"
          />
        </a-col>
        <a-col :xs="24" :sm="12" :md="5">
          <a-space>
            <a-button type="primary" size="large" @click="fetchData">
              <template #icon><SearchOutlined /></template>
              查询
            </a-button>
            <a-button size="large" @click="resetFilter">
              重置
            </a-button>
          </a-space>
        </a-col>
      </a-row>

      <!-- 批量操作工具栏 -->
      <a-row v-if="selectedRowKeys.length > 0" class="mb-3 batch-toolbar">
        <a-col :span="24">
          <a-space>
            <a-alert 
              :message="`已选择 ${selectedRowKeys.length} 项`" 
              type="info"
              show-icon
            >
              <template #action>
                <a-button size="small" type="link" @click="clearSelection">
                  取消选择
                </a-button>
              </template>
            </a-alert>
            <a-popconfirm
              title="确认批量删除选中的告警吗？"
              ok-text="确定"
              cancel-text="取消"
              @confirm="batchDelete"
            >
              <a-button type="primary" danger size="small">
                <template #icon><DeleteOutlined /></template>
                批量删除 ({{ selectedRowKeys.length }})
              </a-button>
            </a-popconfirm>
            <a-button size="small" @click="exportSelected">
              <template #icon><ExportOutlined /></template>
              导出选中
            </a-button>
          </a-space>
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
        :row-selection="{
          selectedRowKeys: selectedRowKeys,
          onChange: onSelectChange,
          selections: [
            {
              key: 'all',
              text: '选择全部',
              onSelect: selectAll,
            },
            {
              key: 'invert',
              text: '反选',
              onSelect: invertSelection,
            },
            {
              key: 'none',
              text: '清空',
              onSelect: clearSelection,
            }
          ]
        }"
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
          <template v-else-if="column.key==='detection_count'">
            <a-badge 
              :count="record.detection_count" 
              :number-style="{ backgroundColor: record.detection_count > 0 ? '#52c41a' : '#d9d9d9' }"
              :show-zero="true"
            />
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
            <a-card title="检测结果可视化" size="small">
              <div style="position: relative; display: inline-block; width: 100%;">
                <img 
                  :src="currentAlert.image_url" 
                  style="width: 100%; display: block;"
                  @load="onImageLoad"
                  ref="alertImage"
                />
                <canvas 
                  ref="canvasRef"
                  style="position: absolute; top: 0; left: 0; pointer-events: none;"
                ></canvas>
              </div>
              <div style="margin-top: 8px; font-size: 12px; color: #666;">
                <a-space direction="vertical" style="width: 100%;">
                  <a-space>
                    <span>检测目标: {{ detections.length }} 个</span>
                    <a-tag v-if="detections.length > 0" color="green">已绘制检测框</a-tag>
                    <a-tag v-else color="orange">无检测结果</a-tag>
                  </a-space>
                  <a-space v-if="algoConfig && algoConfig.regions">
                    <span>配置区域: {{ algoConfig.regions.filter(r => r.enabled).length }} 个</span>
                    <a-tag color="blue">虚线</a-tag>
                  </a-space>
                  <div style="padding: 4px 8px; background: #f0f5ff; border-radius: 4px; border-left: 3px solid #1890ff;">
                    <span style="color: #666;">
                      💡 <strong>图例：</strong>
                      <span style="color: #1890ff;">虚线=配置区域</span> ｜ 
                      <span style="color: #52c41a;">实线=检测结果</span>
                    </span>
                  </div>
                </a-space>
              </div>
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
                <a-descriptions-item label="检测个数">
                  <a-badge 
                    :count="currentAlert.detection_count || 0" 
                    :number-style="{ backgroundColor: '#1890ff' }"
                    :show-zero="true"
                  />
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
            
            <!-- 绊线统计信息卡片 -->
            <a-card v-if="hasLineCrossing" title="绊线统计" size="small" class="mt-3">
              <a-descriptions :column="1" bordered size="small">
                <a-descriptions-item 
                  v-for="(crossing, regionKey) in lineCrossingData" 
                  :key="regionKey"
                  :label="crossing.region_name || regionKey"
                >
                  <a-space>
                    <a-statistic 
                      :value="crossing.count" 
                      :title="'穿越次数'" 
                      :value-style="{ color: '#3f8600', fontSize: '18px' }"
                    />
                    <a-tag :color="crossing.direction === 'in' ? 'blue' : crossing.direction === 'out' ? 'orange' : 'purple'">
                      {{ crossing.direction === 'in' ? '进入' : crossing.direction === 'out' ? '离开' : crossing.direction }}
                    </a-tag>
                  </a-space>
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
import { ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { 
  BellOutlined, ReloadOutlined, ApiOutlined, SearchOutlined,
  EyeOutlined, DeleteOutlined, ExportOutlined
} from '@ant-design/icons-vue'
import { alertApi, frameApi } from '@/api'

const router = useRouter()

const loading = ref(false)
const alerts = ref([])
const taskTypes = ref([])
const taskIds = ref([])
const detailVisible = ref(false)
const currentAlert = ref(null)
const selectedRowKeys = ref([])  // 选中的行ID

// Canvas相关
const canvasRef = ref(null)
const alertImage = ref(null)
const imageLoaded = ref(false)
const detections = ref([])
const lineCrossingData = ref({})
const hasLineCrossing = ref(false)
const algoConfig = ref(null)  // 🔧 算法配置（区域、线条等）

const filter = ref({
  task_id: '',
  task_type: '',
  min_detections: undefined,
  max_detections: undefined,
  page: 1,
  page_size: 20
})

const pagination = ref({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showQuickJumper: true,
  showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
  pageSizeOptions: ['10', '20', '50', '100', '200'],
  size: 'default'
})

const columns = [
  { title: 'ID', key: 'id', width: 80 },
  { title: '任务类型', key: 'task_type', width: 120 },
  { title: '任务ID', key: 'task_id', width: 150 },
  { title: '算法', key: 'algorithm_name', width: 150 },
  { title: '检测数', key: 'detection_count', width: 90 },
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
    pagination.value.pageSize = filter.value.page_size
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

const fetchTaskIds = async () => {
  try {
    const { data } = await alertApi.getTaskIds()
    taskIds.value = data?.task_ids || []
  } catch (e) {
    console.error('fetch task ids failed', e)
  }
}

const filterOption = (input, option) => {
  return option.value.toLowerCase().includes(input.toLowerCase())
}

const handleTableChange = (pag) => {
  filter.value.page = pag.current
  filter.value.page_size = pag.pageSize
  pagination.value.current = pag.current
  pagination.value.pageSize = pag.pageSize
  fetchData()
}

const viewDetail = async (record) => {
  currentAlert.value = record
  detailVisible.value = true
  
  // 解析检测结果
  parseDetections(record)
  
  // 🔧 加载算法配置（用于绘制配置区域）
  await loadAlgoConfig(record.task_id)
}

// 解析检测结果
const parseDetections = (alert) => {
  try {
    if (alert.result) {
      const result = JSON.parse(alert.result)
      detections.value = result.detections || result.objects || []
      
      // 解析绊线统计数据
      if (result.line_crossing) {
        lineCrossingData.value = result.line_crossing
        hasLineCrossing.value = true
      } else {
        lineCrossingData.value = {}
        hasLineCrossing.value = false
      }
    } else {
      detections.value = []
      lineCrossingData.value = {}
      hasLineCrossing.value = false
    }
  } catch (e) {
    console.warn('解析检测结果失败:', e)
    detections.value = []
    lineCrossingData.value = {}
    hasLineCrossing.value = false
  }
}

// 🔧 加载算法配置
const loadAlgoConfig = async (taskId) => {
  try {
    const { data } = await frameApi.getAlgoConfig(taskId)
    algoConfig.value = data
    console.log('已加载算法配置:', data)
  } catch (error) {
    console.log('无算法配置或加载失败:', error)
    algoConfig.value = null
  }
}

// 图片加载完成
const onImageLoad = async () => {
  await nextTick()
  drawAllLayers()  // 🔧 绘制所有图层（配置+检测结果）
}

// 🔧 绘制所有图层（配置区域 + 检测结果）
const drawAllLayers = () => {
  const canvas = canvasRef.value
  const img = alertImage.value
  
  if (!canvas || !img) {
    console.log('绘制条件不满足:', { canvas: !!canvas, img: !!img })
    return
  }
  
  console.log('开始绘制所有图层:', {
    hasAlgoConfig: !!algoConfig.value,
    detectionsCount: detections.value.length,
    imgNaturalSize: { width: img.naturalWidth, height: img.naturalHeight },
    imgDisplaySize: { width: img.offsetWidth, height: img.offsetHeight }
  })
  
  // 设置Canvas尺寸与图片显示尺寸一致
  canvas.width = img.offsetWidth
  canvas.height = img.offsetHeight
  canvas.style.width = img.offsetWidth + 'px'
  canvas.style.height = img.offsetHeight + 'px'
  
  const ctx = canvas.getContext('2d')
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  
  // 🔧 第1层：绘制算法配置区域（底层，半透明）
  if (algoConfig.value && algoConfig.value.regions) {
    drawConfigRegions(ctx, canvas, img)
  }
  
  // 🔧 第2层：绘制检测结果框（上层，高亮）
  if (detections.value.length > 0) {
    drawDetections(ctx, canvas, img)
  }
  
  console.log('所有图层绘制完成')
}

// 🔧 绘制算法配置区域（区域、线条、多边形等）
const drawConfigRegions = (ctx, canvas, img) => {
  const imgNaturalWidth = img.naturalWidth || img.width
  const imgNaturalHeight = img.naturalHeight || img.height
  
  console.log('绘制配置区域:', algoConfig.value.regions.length, '个')
  
  algoConfig.value.regions.forEach((region, index) => {
    if (!region.enabled || !region.points || region.points.length === 0) {
      return
    }
    
    // 将归一化坐标转换为Canvas坐标
    const canvasPoints = region.points.map(point => {
      // 检查是否是归一化坐标
      const isNormalized = point[0] >= 0 && point[0] <= 1 && point[1] >= 0 && point[1] <= 1
      
      if (isNormalized) {
        return [point[0] * canvas.width, point[1] * canvas.height]
      } else {
        // 像素坐标，需要缩放
        const scaleX = canvas.width / imgNaturalWidth
        const scaleY = canvas.height / imgNaturalHeight
        return [point[0] * scaleX, point[1] * scaleY]
      }
    })
    
    const color = region.properties?.color || '#1890ff'
    const opacity = region.properties?.opacity || 0.2
    
    // 根据区域类型绘制
    if (region.type === 'line') {
      // 绘制线条
      ctx.save()
      ctx.strokeStyle = color
      ctx.lineWidth = 3
      ctx.globalAlpha = 0.7
      ctx.setLineDash([5, 5])  // 虚线，与检测结果区分
      ctx.beginPath()
      ctx.moveTo(canvasPoints[0][0], canvasPoints[0][1])
      ctx.lineTo(canvasPoints[1][0], canvasPoints[1][1])
      ctx.stroke()
      
      // 绘制箭头（表示方向）
      const direction = region.properties?.direction || 'in'
      drawDirectionArrow(ctx, canvasPoints, direction, color)
      ctx.restore()
      
    } else if (region.type === 'rectangle') {
      // 绘制矩形
      const [p1, p2] = canvasPoints
      ctx.save()
      ctx.fillStyle = color
      ctx.globalAlpha = opacity
      ctx.fillRect(p1[0], p1[1], p2[0] - p1[0], p2[1] - p1[1])
      ctx.strokeStyle = color
      ctx.globalAlpha = 0.8
      ctx.lineWidth = 2
      ctx.setLineDash([5, 5])
      ctx.strokeRect(p1[0], p1[1], p2[0] - p1[0], p2[1] - p1[1])
      ctx.restore()
      
    } else if (region.type === 'polygon') {
      // 绘制多边形
      ctx.save()
      ctx.fillStyle = color
      ctx.globalAlpha = opacity
      ctx.beginPath()
      ctx.moveTo(canvasPoints[0][0], canvasPoints[0][1])
      for (let i = 1; i < canvasPoints.length; i++) {
        ctx.lineTo(canvasPoints[i][0], canvasPoints[i][1])
      }
      ctx.closePath()
      ctx.fill()
      
      ctx.strokeStyle = color
      ctx.globalAlpha = 0.8
      ctx.lineWidth = 2
      ctx.setLineDash([5, 5])
      ctx.stroke()
      ctx.restore()
    }
    
    // 绘制区域名称标签
    ctx.save()
    ctx.font = '11px Arial'
    ctx.fillStyle = color
    ctx.globalAlpha = 0.9
    const labelX = canvasPoints[0][0]
    const labelY = canvasPoints[0][1] - 5
    ctx.fillText(region.name || `区域${index + 1}`, labelX, labelY)
    ctx.restore()
  })
}

// 🔧 绘制方向箭头（用于线条）
const drawDirectionArrow = (ctx, points, direction, color) => {
  const [p1, p2] = points
  const lineAngle = Math.atan2(p2[1] - p1[1], p2[0] - p1[0])
  const midX = (p1[0] + p2[0]) / 2
  const midY = (p1[1] + p2[1]) / 2
  
  const perpAngleDown = lineAngle + Math.PI / 2
  const perpAngleUp = lineAngle - Math.PI / 2
  
  const drawArrow = (x, y, angle) => {
    const size = 10
    ctx.beginPath()
    ctx.moveTo(x, y)
    ctx.lineTo(x - size * Math.cos(angle - Math.PI / 6), y - size * Math.sin(angle - Math.PI / 6))
    ctx.moveTo(x, y)
    ctx.lineTo(x - size * Math.cos(angle + Math.PI / 6), y - size * Math.sin(angle + Math.PI / 6))
    ctx.stroke()
  }
  
  if (direction === 'in') {
    drawArrow(midX, midY, perpAngleDown)
  } else if (direction === 'out') {
    drawArrow(midX, midY, perpAngleUp)
  } else if (direction === 'in_out') {
    const offset = 15
    const offsetX = offset * Math.cos(lineAngle)
    const offsetY = offset * Math.sin(lineAngle)
    drawArrow(midX - offsetX, midY - offsetY, perpAngleDown)
    drawArrow(midX + offsetX, midY + offsetY, perpAngleUp)
  }
}

// 绘制检测框
const drawDetections = (ctx, canvas, img) => {
  const imgNaturalWidth = img.naturalWidth || img.width
  const imgNaturalHeight = img.naturalHeight || img.height
  
  // 计算缩放比例（原始图片尺寸 -> 显示尺寸）
  const scaleX = canvas.width / imgNaturalWidth
  const scaleY = canvas.height / imgNaturalHeight
  
  console.log('绘制检测框:', detections.value.length, '个')
  
  // 绘制每个检测框
  detections.value.forEach((detection, index) => {
    if (!detection.bbox || detection.bbox.length < 4) {
      console.warn(`检测框 ${index} 数据无效:`, detection)
      return
    }
    
    // bbox格式: [x1, y1, x2, y2] - 左上角和右下角坐标（像素坐标）
    const [x1, y1, x2, y2] = detection.bbox
    const confidence = detection.confidence || detection.score || 0.5
    const className = detection.class_name || detection.class || `目标${index + 1}`
    
    // 计算在Canvas上的位置
    let canvasX1, canvasY1, canvasX2, canvasY2, canvasW, canvasH
    
    // 判断是否为归一化坐标（0-1之间）
    const isNormalized = x1 <= 1 && y1 <= 1 && x2 <= 1 && y2 <= 1
    
    if (isNormalized) {
      // 归一化坐标，直接乘以Canvas尺寸
      canvasX1 = x1 * canvas.width
      canvasY1 = y1 * canvas.height
      canvasX2 = x2 * canvas.width
      canvasY2 = y2 * canvas.height
    } else {
      // 像素坐标（基于原始图片尺寸），按比例缩放到显示尺寸
      canvasX1 = x1 * scaleX
      canvasY1 = y1 * scaleY
      canvasX2 = x2 * scaleX
      canvasY2 = y2 * scaleY
    }
    
    // 计算宽高
    canvasW = canvasX2 - canvasX1
    canvasH = canvasY2 - canvasY1
    
    // 确保坐标有效
    if (canvasW <= 0 || canvasH <= 0) {
      console.warn(`检测框 ${index} 尺寸无效:`, { canvasW, canvasH })
      return
    }
    
    // 绘制检测框（实线，高亮）
    ctx.strokeStyle = confidence > 0.8 ? '#52c41a' : '#faad14'
    ctx.lineWidth = 3  // 比配置区域粗
    ctx.setLineDash([])  // 实线
    ctx.strokeRect(canvasX1, canvasY1, canvasW, canvasH)
    
    // 绘制标签背景
    const labelText = `${className} ${(confidence * 100).toFixed(1)}%`
    ctx.font = '13px Arial'
    const labelWidth = ctx.measureText(labelText).width + 10
    const labelHeight = 22
    
    ctx.fillStyle = confidence > 0.8 ? '#52c41a' : '#faad14'
    ctx.fillRect(canvasX1, canvasY1 - labelHeight, labelWidth, labelHeight)
    
    // 绘制标签文字
    ctx.fillStyle = '#fff'
    ctx.fillText(labelText, canvasX1 + 5, canvasY1 - 6)
  })
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

const resetFilter = () => {
  filter.value = {
    task_id: '',
    task_type: '',
    min_detections: undefined,
    max_detections: undefined,
    page: 1,
    page_size: 20
  }
  pagination.value.current = 1
  pagination.value.pageSize = 20
  fetchData()
}

// ========== 批量操作相关函数 ==========

// 选择变化时
const onSelectChange = (keys) => {
  selectedRowKeys.value = keys
}

// 全选
const selectAll = () => {
  selectedRowKeys.value = alerts.value.map(item => item.id)
}

// 反选
const invertSelection = () => {
  const allKeys = alerts.value.map(item => item.id)
  selectedRowKeys.value = allKeys.filter(key => !selectedRowKeys.value.includes(key))
}

// 清空选择
const clearSelection = () => {
  selectedRowKeys.value = []
}

// 批量删除
const batchDelete = async () => {
  if (selectedRowKeys.value.length === 0) {
    message.warning('请先选择要删除的告警')
    return
  }
  
  loading.value = true
  try {
    // 调用批量删除API
    await alertApi.batchDeleteAlerts(selectedRowKeys.value)
    message.success(`成功删除 ${selectedRowKeys.value.length} 条告警`)
    clearSelection()
    fetchData()
  } catch (e) {
    console.error('batch delete failed', e)
    message.error('批量删除失败: ' + (e.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

// 导出选中项
const exportSelected = () => {
  if (selectedRowKeys.value.length === 0) {
    message.warning('请先选择要导出的告警')
    return
  }
  
  try {
    const selectedAlerts = alerts.value.filter(item => selectedRowKeys.value.includes(item.id))
    
    // 构建CSV内容
    const headers = ['ID', '任务类型', '任务ID', '算法', '检测数', '置信度', '推理时间(ms)', '图片路径', '告警时间']
    const rows = selectedAlerts.map(item => [
      item.id,
      item.task_type,
      item.task_id,
      item.algorithm_name,
      item.detection_count,
      item.confidence,
      item.inference_time_ms,
      item.image_path,
      formatTime(item.created_at)
    ])
    
    const csvContent = [
      headers.join(','),
      ...rows.map(row => row.map(cell => `"${cell}"`).join(','))
    ].join('\n')
    
    // 添加BOM头以支持中文
    const BOM = '\uFEFF'
    const blob = new Blob([BOM + csvContent], { type: 'text/csv;charset=utf-8;' })
    const link = document.createElement('a')
    const url = URL.createObjectURL(blob)
    
    link.setAttribute('href', url)
    link.setAttribute('download', `alerts_${new Date().getTime()}.csv`)
    link.style.visibility = 'hidden'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    
    message.success('导出成功')
  } catch (e) {
    console.error('export failed', e)
    message.error('导出失败')
  }
}

onMounted(() => {
  fetchData()
  fetchTaskTypes()
  fetchTaskIds()
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

.mb-3 {
  margin-bottom: 12px;
}

.batch-toolbar {
  padding: 12px;
  background: #e6f7ff;
  border: 1px solid #91d5ff;
  border-radius: 4px;
  transition: all 0.3s;
}

.batch-toolbar :deep(.ant-alert) {
  border: none;
  background: transparent;
}

/* 检测框Canvas样式 */
canvas {
  border-radius: 4px;
}

/* 检测结果标签样式 */
.text-gray-500 {
  color: #999;
}

.text-xs {
  font-size: 12px;
}
</style>

