<template>
  <a-modal
    v-model:visible="visible"
    title="算法配置"
    width="95%"
    :footer="null"
    :destroyOnClose="true"
    @cancel="handleClose"
  >
    <div class="algo-config-container">
      <!-- 左侧画布区域 -->
      <div class="canvas-area">
        <a-card title="绘图区域" size="small">
          <template #extra>
            <a-space>
              <a-tag color="blue">{{ taskInfo.task_type }}</a-tag>
              <a-tag :color="regions.length > 0 ? 'green' : 'orange'">
                {{ regions.length }}个区域
              </a-tag>
            </a-space>
          </template>
          
          <!-- 工具栏 -->
          <div class="toolbar">
            <a-space>
              <a-button-group>
                <a-button 
                  :type="drawMode === 'line' ? 'primary' : 'default'"
                  @click="setDrawMode('line')"
                >
                  <template #icon><LineOutlined /></template>
                  绘制线
                </a-button>
                <a-button 
                  :type="drawMode === 'rect' ? 'primary' : 'default'"
                  @click="setDrawMode('rect')"
                >
                  <template #icon><BorderOutlined /></template>
                  绘制矩形
                </a-button>
                <a-button 
                  :type="drawMode === 'polygon' ? 'primary' : 'default'"
                  @click="setDrawMode('polygon')"
                >
                  <template #icon><AppstoreOutlined /></template>
                  绘制多边形
                </a-button>
              </a-button-group>
              
              <a-divider type="vertical" />
              
              <a-button @click="deleteSelected" danger>
                <template #icon><DeleteOutlined /></template>
                删除选中
              </a-button>
              
              <a-button @click="clearAll">
                <template #icon><ClearOutlined /></template>
                清空全部
              </a-button>
              
              <a-divider type="vertical" />
              
              <a-button @click="resetCanvas">
                <template #icon><ReloadOutlined /></template>
                重置
              </a-button>
            </a-space>
          </div>
          
          <!-- Canvas画布 -->
          <div class="canvas-wrapper">
            <canvas id="algo-canvas" ref="canvasRef"></canvas>
          </div>
          
          <!-- 提示信息 -->
          <a-alert 
            v-if="drawMode === 'polygon'" 
            message="多边形绘制：左键点击添加点，双击或右键完成绘制"
            type="info"
            show-icon
            closable
            style="margin-top: 8px"
          />
        </a-card>
      </div>
      
      <!-- 右侧配置面板 -->
      <div class="config-panel">
        <a-card title="区域配置" size="small">
          <template #extra>
            <a-button type="primary" @click="saveConfig" :loading="saving">
              <template #icon><SaveOutlined /></template>
              保存配置
            </a-button>
          </template>
          
          <!-- 区域列表 -->
          <div class="regions-list">
            <a-empty v-if="regions.length === 0" description="暂无区域，请在左侧画布绘制" />
            
            <a-collapse v-else v-model:activeKey="activeRegion" accordion>
              <a-collapse-panel 
                v-for="(region, index) in regions" 
                :key="region.id"
                :header="`${region.name}`"
              >
                <template #extra>
                  <a-space>
                    <a-tag :color="getTypeColor(region.type)">
                      {{ getTypeLabel(region.type) }}
                    </a-tag>
                    <a-switch 
                      v-model:checked="region.enabled" 
                      size="small"
                      checked-children="启用"
                      un-checked-children="禁用"
                      @click.stop
                    />
                  </a-space>
                </template>
                
                <!-- 区域详细配置 -->
                <a-form layout="vertical" size="small">
                  <a-form-item label="区域名称">
                    <a-input v-model:value="region.name" />
                  </a-form-item>
                  
                  <a-form-item label="区域类型">
                    <a-select v-model:value="region.type" disabled>
                      <a-select-option value="line">线</a-select-option>
                      <a-select-option value="rectangle">矩形</a-select-option>
                      <a-select-option value="polygon">多边形</a-select-option>
                    </a-select>
                  </a-form-item>
                  
                  <a-form-item label="颜色">
                    <input 
                      type="color" 
                      v-model="region.properties.color"
                      @change="updateRegionStyle(region)"
                      style="width: 100%; height: 32px; cursor: pointer;"
                    />
                  </a-form-item>
                  
                  <a-form-item label="透明度">
                    <a-slider 
                      v-model:value="region.properties.opacity" 
                      :min="0" 
                      :max="1" 
                      :step="0.1"
                      @change="updateRegionStyle(region)"
                    />
                  </a-form-item>
                  
                  <a-form-item label="检测阈值" v-if="region.type !== 'line'">
                    <a-input-number 
                      v-model:value="region.properties.threshold" 
                      :min="0" 
                      :max="1" 
                      :step="0.05"
                      style="width: 100%"
                    />
                  </a-form-item>
                  
                  <a-form-item label="方向" v-if="region.type === 'line'">
                    <a-select v-model:value="region.properties.direction">
                      <a-select-option value="bidirectional">双向</a-select-option>
                      <a-select-option value="in">进入</a-select-option>
                      <a-select-option value="out">离开</a-select-option>
                    </a-select>
                  </a-form-item>
                  
                  <a-form-item label="坐标点">
                    <a-textarea 
                      :value="formatPoints(region.points)" 
                      :rows="3" 
                      disabled
                    />
                  </a-form-item>
                  
                  <a-button 
                    danger 
                    block 
                    @click="deleteRegion(region.id)"
                  >
                    <template #icon><DeleteOutlined /></template>
                    删除此区域
                  </a-button>
                </a-form>
              </a-collapse-panel>
            </a-collapse>
          </div>
          
          <!-- 算法参数 -->
          <a-divider>算法参数</a-divider>
          <a-form layout="vertical" size="small">
            <a-form-item label="置信度阈值">
              <a-input-number 
                v-model:value="algorithmParams.confidence_threshold" 
                :min="0" 
                :max="1" 
                :step="0.05"
                style="width: 100%"
              />
            </a-form-item>
            
            <a-form-item label="IOU阈值">
              <a-input-number 
                v-model:value="algorithmParams.iou_threshold" 
                :min="0" 
                :max="1" 
                :step="0.05"
                style="width: 100%"
              />
            </a-form-item>
          </a-form>
        </a-card>
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { ref, onMounted, watch, nextTick } from 'vue'
import { message } from 'ant-design-vue'
import { 
  LineOutlined, BorderOutlined, AppstoreOutlined,
  DeleteOutlined, ClearOutlined, ReloadOutlined, SaveOutlined
} from '@ant-design/icons-vue'
import { fabric } from 'fabric'
import { frameApi } from '@/api'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  taskInfo: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['update:modelValue', 'saved'])

const visible = ref(false)
const canvasRef = ref(null)
let canvas = null
let backgroundImage = null

const drawMode = ref(null) // 'line' | 'rect' | 'polygon'
const regions = ref([])
const activeRegion = ref([])
const saving = ref(false)
const polygonPoints = ref([])
const tempPolygonLine = ref(null)

const algorithmParams = ref({
  confidence_threshold: 0.7,
  iou_threshold: 0.5
})

// 监听visible变化
watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    nextTick(() => {
      initCanvas()
    })
  }
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

// 初始化Canvas
const initCanvas = async () => {
  try {
    // 创建Fabric Canvas
    canvas = new fabric.Canvas('algo-canvas', {
      width: 800,
      height: 600,
      backgroundColor: '#f0f0f0',
      selection: true
    })
    
    // 加载预览图片
    await loadPreviewImage()
    
    // 加载已有配置
    await loadExistingConfig()
    
    // 绑定事件
    bindCanvasEvents()
    
  } catch (error) {
    console.error('初始化Canvas失败:', error)
    message.error('初始化失败: ' + error.message)
  }
}

// 加载预览图片
const loadPreviewImage = async () => {
  try {
    const { data } = await frameApi.getPreviewImage(props.taskInfo.id)
    if (data && data.preview_image) {
      // 构建图片URL（需要通过MinIO或后端代理）
      const imageUrl = `/api/minio/preview/${data.preview_image}`
      
      fabric.Image.fromURL(imageUrl, (img) => {
        if (!img) {
          message.warning('预览图片加载失败，请确保已抽取预览帧')
          return
        }
        
        // 缩放图片适应画布
        const scale = Math.min(
          canvas.width / img.width,
          canvas.height / img.height
        )
        
        img.scale(scale)
        img.set({
          left: 0,
          top: 0,
          selectable: false,
          evented: false
        })
        
        backgroundImage = img
        canvas.setBackgroundImage(img, canvas.renderAll.bind(canvas))
      }, { crossOrigin: 'anonymous' })
    }
  } catch (error) {
    console.error('加载预览图片失败:', error)
  }
}

// 加载已有配置
const loadExistingConfig = async () => {
  try {
    const { data } = await frameApi.getAlgoConfig(props.taskInfo.id)
    if (data && data.regions) {
      regions.value = data.regions
      algorithmParams.value = data.algorithm_params || algorithmParams.value
      
      // 在画布上绘制已有区域
      regions.value.forEach(region => {
        drawRegionOnCanvas(region)
      })
    }
  } catch (error) {
    console.log('无已有配置:', error)
  }
}

// 在画布上绘制区域
const drawRegionOnCanvas = (region) => {
  let shape = null
  
  if (region.type === 'line') {
    shape = new fabric.Line(
      [region.points[0][0], region.points[0][1], region.points[1][0], region.points[1][1]],
      {
        stroke: region.properties.color,
        strokeWidth: region.properties.thickness || 3,
        selectable: true,
        hasControls: true
      }
    )
  } else if (region.type === 'rectangle') {
    const [p1, p2] = region.points
    shape = new fabric.Rect({
      left: p1[0],
      top: p1[1],
      width: p2[0] - p1[0],
      height: p2[1] - p1[1],
      fill: region.properties.color,
      opacity: region.properties.opacity || 0.3,
      stroke: region.properties.color,
      strokeWidth: 2,
      selectable: true
    })
  } else if (region.type === 'polygon') {
    shape = new fabric.Polygon(
      region.points.map(p => ({ x: p[0], y: p[1] })),
      {
        fill: region.properties.color,
        opacity: region.properties.opacity || 0.3,
        stroke: region.properties.color,
        strokeWidth: 2,
        selectable: true
      }
    )
  }
  
  if (shape) {
    shape.set('regionId', region.id)
    canvas.add(shape)
  }
}

// 设置绘制模式
const setDrawMode = (mode) => {
  if (drawMode.value === mode) {
    drawMode.value = null
    canvas.isDrawingMode = false
  } else {
    drawMode.value = mode
    canvas.isDrawingMode = false
    
    if (mode === 'polygon') {
      polygonPoints.value = []
      message.info('点击画布添加多边形顶点，双击完成绘制')
    }
  }
}

// 绑定Canvas事件
const bindCanvasEvents = () => {
  canvas.on('mouse:down', handleCanvasMouseDown)
  canvas.on('mouse:dblclick', handleCanvasDoubleClick)
}

// Canvas鼠标点击
const handleCanvasMouseDown = (e) => {
  if (!drawMode.value) return
  
  const pointer = canvas.getPointer(e.e)
  
  if (drawMode.value === 'line') {
    drawLine(pointer)
  } else if (drawMode.value === 'rect') {
    drawRectangle(pointer)
  } else if (drawMode.value === 'polygon') {
    addPolygonPoint(pointer)
  }
}

// Canvas双击
const handleCanvasDoubleClick = () => {
  if (drawMode.value === 'polygon' && polygonPoints.value.length >= 3) {
    finishPolygon()
  }
}

// 绘制线
let lineStart = null
const drawLine = (pointer) => {
  if (!lineStart) {
    lineStart = pointer
  } else {
    const line = new fabric.Line([lineStart.x, lineStart.y, pointer.x, pointer.y], {
      stroke: '#FF0000',
      strokeWidth: 3,
      selectable: true
    })
    
    const regionId = `region_${Date.now()}`
    line.set('regionId', regionId)
    canvas.add(line)
    
    // 添加到区域列表
    regions.value.push({
      id: regionId,
      name: `线_${regions.value.length + 1}`,
      type: 'line',
      enabled: true,
      points: [[lineStart.x, lineStart.y], [pointer.x, pointer.y]],
      properties: {
        color: '#FF0000',
        thickness: 3,
        direction: 'bidirectional'
      }
    })
    
    lineStart = null
    drawMode.value = null
    message.success('线绘制完成')
  }
}

// 绘制矩形
let rectStart = null
const drawRectangle = (pointer) => {
  if (!rectStart) {
    rectStart = pointer
  } else {
    const left = Math.min(rectStart.x, pointer.x)
    const top = Math.min(rectStart.y, pointer.y)
    const width = Math.abs(pointer.x - rectStart.x)
    const height = Math.abs(pointer.y - rectStart.y)
    
    const rect = new fabric.Rect({
      left,
      top,
      width,
      height,
      fill: '#00FF00',
      opacity: 0.3,
      stroke: '#00FF00',
      strokeWidth: 2,
      selectable: true
    })
    
    const regionId = `region_${Date.now()}`
    rect.set('regionId', regionId)
    canvas.add(rect)
    
    // 添加到区域列表
    regions.value.push({
      id: regionId,
      name: `矩形_${regions.value.length + 1}`,
      type: 'rectangle',
      enabled: true,
      points: [[left, top], [left + width, top + height]],
      properties: {
        color: '#00FF00',
        opacity: 0.3,
        threshold: 0.5
      }
    })
    
    rectStart = null
    drawMode.value = null
    message.success('矩形绘制完成')
  }
}

// 添加多边形顶点
const addPolygonPoint = (pointer) => {
  polygonPoints.value.push(pointer)
  
  // 绘制临时点
  const circle = new fabric.Circle({
    left: pointer.x - 3,
    top: pointer.y - 3,
    radius: 3,
    fill: '#0000FF',
    selectable: false
  })
  canvas.add(circle)
}

// 完成多边形绘制
const finishPolygon = () => {
  if (polygonPoints.value.length < 3) {
    message.error('多边形至少需要3个顶点')
    return
  }
  
  // 清除临时点
  canvas.getObjects('circle').forEach(obj => canvas.remove(obj))
  
  const polygon = new fabric.Polygon(
    polygonPoints.value.map(p => ({ x: p.x, y: p.y })),
    {
      fill: '#0000FF',
      opacity: 0.3,
      stroke: '#0000FF',
      strokeWidth: 2,
      selectable: true
    }
  )
  
  const regionId = `region_${Date.now()}`
  polygon.set('regionId', regionId)
  canvas.add(polygon)
  
  // 添加到区域列表
  regions.value.push({
    id: regionId,
    name: `多边形_${regions.value.length + 1}`,
    type: 'polygon',
    enabled: true,
    points: polygonPoints.value.map(p => [Math.round(p.x), Math.round(p.y)]),
    properties: {
      color: '#0000FF',
      opacity: 0.3,
      threshold: 0.5
    }
  })
  
  polygonPoints.value = []
  drawMode.value = null
  message.success('多边形绘制完成')
}

// 删除选中
const deleteSelected = () => {
  const activeObjects = canvas.getActiveObjects()
  if (activeObjects.length === 0) {
    message.warning('请先选中要删除的区域')
    return
  }
  
  activeObjects.forEach(obj => {
    const regionId = obj.get('regionId')
    if (regionId) {
      deleteRegion(regionId)
    }
    canvas.remove(obj)
  })
  canvas.discardActiveObject()
  canvas.renderAll()
}

// 删除区域
const deleteRegion = (regionId) => {
  regions.value = regions.value.filter(r => r.id !== regionId)
  
  // 从画布删除
  const obj = canvas.getObjects().find(o => o.get('regionId') === regionId)
  if (obj) {
    canvas.remove(obj)
    canvas.renderAll()
  }
  
  message.success('区域已删除')
}

// 清空全部
const clearAll = () => {
  canvas.getObjects().forEach(obj => {
    if (obj.get('regionId')) {
      canvas.remove(obj)
    }
  })
  regions.value = []
  canvas.renderAll()
  message.success('已清空所有区域')
}

// 重置画布
const resetCanvas = () => {
  clearAll()
  loadExistingConfig()
}

// 更新区域样式
const updateRegionStyle = (region) => {
  const obj = canvas.getObjects().find(o => o.get('regionId') === region.id)
  if (obj) {
    if (region.type === 'line') {
      obj.set('stroke', region.properties.color)
    } else {
      obj.set({
        fill: region.properties.color,
        stroke: region.properties.color,
        opacity: region.properties.opacity
      })
    }
    canvas.renderAll()
  }
}

// 格式化坐标点
const formatPoints = (points) => {
  return JSON.stringify(points, null, 2)
}

// 获取类型颜色
const getTypeColor = (type) => {
  const colors = {
    line: 'red',
    rectangle: 'green',
    polygon: 'blue'
  }
  return colors[type] || 'default'
}

// 获取类型标签
const getTypeLabel = (type) => {
  const labels = {
    line: '线',
    rectangle: '矩形',
    polygon: '多边形'
  }
  return labels[type] || type
}

// 保存配置
const saveConfig = async () => {
  if (regions.value.length === 0) {
    message.warning('请至少绘制一个区域')
    return
  }
  
  saving.value = true
  try {
    const config = {
      task_id: props.taskInfo.id,
      task_type: props.taskInfo.task_type,
      config_version: '1.0',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      regions: regions.value,
      algorithm_params: algorithmParams.value
    }
    
    await frameApi.saveAlgoConfig(props.taskInfo.id, config)
    message.success('配置保存成功')
    emit('saved')
    handleClose()
  } catch (error) {
    console.error('保存配置失败:', error)
    message.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    saving.value = false
  }
}

// 关闭弹窗
const handleClose = () => {
  visible.value = false
  if (canvas) {
    canvas.dispose()
    canvas = null
  }
}

onMounted(() => {
  visible.value = props.modelValue
})
</script>

<style scoped>
.algo-config-container {
  display: flex;
  gap: 16px;
  height: 70vh;
}

.canvas-area {
  flex: 1;
  min-width: 0;
}

.config-panel {
  width: 350px;
  overflow-y: auto;
}

.toolbar {
  margin-bottom: 12px;
}

.canvas-wrapper {
  position: relative;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  overflow: hidden;
  background: #f0f0f0;
}

#algo-canvas {
  display: block;
}

.regions-list {
  max-height: 400px;
  overflow-y: auto;
}

:deep(.ant-collapse-item) {
  margin-bottom: 8px;
}
</style>

