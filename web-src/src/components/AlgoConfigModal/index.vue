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
              <a-tag color="cyan">{{ taskInfo.id }}</a-tag>
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
              
              <a-button @click="deleteSelected" danger :disabled="!canvas">
                <template #icon><DeleteOutlined /></template>
                删除选中
              </a-button>
              
              <a-button @click="clearAll" :disabled="regions.length === 0">
                <template #icon><ClearOutlined /></template>
                清空全部
              </a-button>
              
              <a-divider type="vertical" />
              
              <a-button @click="resetCanvas" :disabled="!canvas">
                <template #icon><ReloadOutlined /></template>
                重置
              </a-button>
            </a-space>
          </div>
          
          <!-- 状态提示 -->
          <a-alert 
            v-if="!backgroundImage && !imageLoading"
            message="预览图片未加载"
            description="请确保任务已生成预览图。如果预览图不显示，请稍候刷新或重新添加任务。"
            type="warning"
            show-icon
            style="margin-bottom: 12px"
          />
          
          <a-spin :spinning="imageLoading" tip="正在加载预览图片...">
            <!-- Canvas画布 -->
            <div class="canvas-wrapper" :class="{ 'has-image': backgroundImage }">
              <canvas id="algo-canvas" ref="canvasRef"></canvas>
              <div v-if="!backgroundImage && !imageLoading" class="canvas-placeholder">
                <PictureOutlined style="font-size: 48px; color: #d9d9d9;" />
                <div style="margin-top: 8px; color: #999;">等待预览图片加载...</div>
              </div>
            </div>
          </a-spin>
          
          <!-- 提示信息 -->
          <a-alert 
            v-if="drawMode === 'polygon'" 
            message="多边形绘制：左键点击添加点，双击或右键完成绘制"
            type="info"
            show-icon
            closable
            style="margin-top: 8px"
          />
          <a-alert 
            v-if="drawMode === 'line'" 
            message="线段绘制：点击起点，再点击终点完成"
            type="info"
            show-icon
            closable
            style="margin-top: 8px"
          />
          <a-alert 
            v-if="drawMode === 'rect'" 
            message="矩形绘制：点击一个角，再点击对角完成"
            type="info"
            show-icon
            closable
            style="margin-top: 8px"
          />
        </a-card>
      </div>
      
      <!-- 右侧配置面板 -->
      <div class="config-panel">
        <!-- 图片信息 -->
        <a-card size="small" class="mb-3" v-if="backgroundImage">
          <a-descriptions size="small" :column="1" bordered>
            <a-descriptions-item label="预览图">
              <a-tag color="green">已加载</a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="分辨率">
              {{ Math.floor(backgroundImage.width) }} x {{ Math.floor(backgroundImage.height) }}
            </a-descriptions-item>
            <a-descriptions-item label="画布尺寸">
              {{ canvas ? canvas.width : 0 }} x {{ canvas ? canvas.height : 0 }}
            </a-descriptions-item>
          </a-descriptions>
        </a-card>
        
        <a-card title="区域配置" size="small">
          <template #extra>
            <a-button type="primary" @click="saveConfig" :loading="saving" :disabled="!backgroundImage">
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
                  
                  <a-form-item label="检测方向" v-if="region.type === 'line'">
                    <a-select 
                      v-model:value="region.properties.direction"
                      @change="updateRegionArrow(region)"
                    >
                      <a-select-option value="in">
                        <span>⬇ 进入（上→下穿过）</span>
                      </a-select-option>
                      <a-select-option value="out">
                        <span>⬆ 离开（下→上穿过）</span>
                      </a-select-option>
                      <a-select-option value="in_out">
                        <span>⬍ 进出（双向穿过）</span>
                      </a-select-option>
                    </a-select>
                    <div style="margin-top: 8px; font-size: 12px; color: #999;">
                      箭头垂直于线条，表示穿越方向
                    </div>
                  </a-form-item>
                  
                  <a-form-item label="坐标点">
                    <a-tabs size="small" type="card">
                      <a-tab-pane key="pixel" tab="像素坐标">
                        <a-textarea 
                          :value="formatPoints(region.points)" 
                          :rows="3" 
                          disabled
                        />
                      </a-tab-pane>
                      <a-tab-pane key="normalized" tab="归一化坐标">
                        <a-textarea 
                          :value="formatNormalizedPoints(region.points)" 
                          :rows="3" 
                          disabled
                        />
                      </a-tab-pane>
                    </a-tabs>
                    <div style="margin-top: 8px; font-size: 12px; color: #999;">
                      💡 保存时使用归一化坐标，与图片分辨率无关
                    </div>
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
                :precision="2"
                style="width: 100%"
                placeholder="0.05"
              >
                <template #addonAfter>
                  <a-tooltip title="检测结果置信度低于此值将被过滤">
                    <InfoCircleOutlined />
                  </a-tooltip>
                </template>
              </a-input-number>
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
            <a-form-item label="保存告警图片">
              <a-switch 
                v-model:checked="saveAlertImage" 
                checked-children="保存" 
                un-checked-children="不保存" 
              />
              <div style="margin-top: 8px; font-size: 12px; color: #999;">
                默认关闭。如需在告警中心查看此任务的图片，请开启后重新保存配置。
              </div>
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
  DeleteOutlined, ClearOutlined, ReloadOutlined, SaveOutlined,
  PictureOutlined, InfoCircleOutlined
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
let canvasWidth = 0  // 画布实际宽度
let canvasHeight = 0 // 画布实际高度

const drawMode = ref(null) // 'line' | 'rect' | 'polygon'
const regions = ref([])
const activeRegion = ref([])
const saving = ref(false)
const imageLoading = ref(false)
const polygonPoints = ref([])
const tempPolygonLine = ref(null)

const algorithmParams = ref({
  confidence_threshold: 0.05,  // 🔧 默认置信度改为0.05
  iou_threshold: 0.5
})
const saveAlertImage = ref(false) // 是否保存告警图片

// 监听visible变化
watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    // 🔧 确保每次打开时都完全重新初始化
    nextTick(async () => {
      // 清理旧Canvas（如果存在）
      if (canvas) {
        canvas.dispose()
        canvas = null
        backgroundImage = null
      }
      
      // 重置状态
      regions.value = []
      activeRegion.value = []
      drawMode.value = null
      polygonPoints.value = []
      saveAlertImage.value = false
      
      // 初始化新Canvas
      await initCanvas()
    })
  }
})

watch(visible, (val) => {
  emit('update:modelValue', val)
  
  // 🔧 关闭时清理Canvas
  if (!val && canvas) {
    canvas.dispose()
    canvas = null
    backgroundImage = null
    canvasWidth = 0
    canvasHeight = 0
    regions.value = []
  }
})

// 初始化Canvas
const initCanvas = async () => {
  try {
    // 创建Fabric Canvas（初始尺寸，加载图片后会调整）
    canvas = new fabric.Canvas('algo-canvas', {
      width: 800,
      height: 600,
      backgroundColor: '#f5f5f5',
      selection: true,
      preserveObjectStacking: true
    })
    
    console.log('Canvas initialized, loading preview image...')
    
    // 加载预览图片（会自动调整画布尺寸）
    await loadPreviewImage()
    
    // 加载已有配置
    await loadExistingConfig()
    
    // 绑定事件
    bindCanvasEvents()
    
    console.log('Canvas setup complete')
    
  } catch (error) {
    console.error('初始化Canvas失败:', error)
    message.error('初始化失败: ' + error.message)
  }
}

// 加载预览图片
const loadPreviewImage = async () => {
  imageLoading.value = true
  try {
    const { data } = await frameApi.getPreviewImage(props.taskInfo.id)
    if (data && data.preview_image) {
      // 构建图片URL（通过后端MinIO代理）
      const imageUrl = `/api/v1/minio/preview/${data.preview_image}`
      
      console.log('Loading preview image from:', imageUrl)
      
      // 🔧 将fabric.Image.fromURL包装成Promise，确保真正等待图片加载完成
      await new Promise((resolve, reject) => {
        fabric.Image.fromURL(imageUrl, (img) => {
          imageLoading.value = false
          
          if (!img || img.width === 0) {
            const error = new Error('预览图片加载失败，请检查图片是否存在')
            message.error(error.message)
            console.error('Image load failed or empty:', imageUrl)
            reject(error)
            return
          }
          
          // 根据图片尺寸调整画布大小（保持最大800x600）
          const maxWidth = 800
          const maxHeight = 600
          const scale = Math.min(
            maxWidth / img.width,
            maxHeight / img.height,
            1  // 不放大，只缩小
          )
          
          const canvasWidthCalc = Math.floor(img.width * scale)
          const canvasHeightCalc = Math.floor(img.height * scale)
          
          // 🔧 保存画布尺寸到全局变量（关键：必须在这里设置，loadExistingConfig依赖这些值）
          canvasWidth = canvasWidthCalc
          canvasHeight = canvasHeightCalc
          
          console.log('🔧 Canvas尺寸已设置:', { canvasWidth, canvasHeight })
          
          // 设置画布尺寸
          canvas.setDimensions({
            width: canvasWidthCalc,
            height: canvasHeightCalc
          })
          
          // 缩放图片
          img.scale(scale)
          img.set({
            left: 0,
            top: 0,
            selectable: false,
            evented: false,
            hasControls: false,
            hasBorders: false,
            lockMovementX: true,
            lockMovementY: true
          })
          
          backgroundImage = img
          canvas.setBackgroundImage(img, canvas.renderAll.bind(canvas))
          
          message.success(`预览图片加载成功 (${img.width}x${img.height})`)
          console.log('Preview image loaded successfully:', {
            original: `${img.width}x${img.height}`,
            canvas: `${canvasWidth}x${canvasHeight}`,
            scale: scale
          })
          
          // 🔧 图片加载完成，Promise resolve
          resolve()
        }, { 
          crossOrigin: 'anonymous'
        })
      })
    } else {
      imageLoading.value = false
      message.warning('预览图片尚未生成，请等待或重新添加任务')
    }
  } catch (error) {
    imageLoading.value = false
    console.error('加载预览图片失败:', error)
    message.error('加载预览图片失败: ' + (error.response?.data?.error || error.message))
    throw error  // 🔧 抛出错误，阻止后续加载配置
  }
}

// ==================== 坐标转换函数 ====================

// 将归一化坐标转换为像素坐标
const normalizedToPixel = (normalizedPoints) => {
  if (!canvasWidth || !canvasHeight) {
    console.warn('画布尺寸未初始化')
    return normalizedPoints
  }
  
  return normalizedPoints.map(point => {
    if (Array.isArray(point) && point.length === 2) {
      return [
        Math.round(point[0] * canvasWidth),
        Math.round(point[1] * canvasHeight)
      ]
    }
    return point
  })
}

// 将像素坐标转换为归一化坐标
const pixelToNormalized = (pixelPoints) => {
  if (!canvasWidth || !canvasHeight) {
    console.warn('画布尺寸未初始化')
    return pixelPoints
  }
  
  return pixelPoints.map(point => {
    if (Array.isArray(point) && point.length === 2) {
      return [
        Math.round((point[0] / canvasWidth) * 10000) / 10000,  // 保留4位小数
        Math.round((point[1] / canvasHeight) * 10000) / 10000
      ]
    }
    return point
  })
}

// 加载已有配置
const loadExistingConfig = async () => {
  try {
    console.log('开始加载已有配置...')
    const { data } = await frameApi.getAlgoConfig(props.taskInfo.id)
    
    if (data && data.regions) {
      console.log('获取到配置:', data.regions.length, '个区域')
      
      // 🔧 深拷贝配置，避免修改原数据
      regions.value = JSON.parse(JSON.stringify(data.regions))
      algorithmParams.value = data.algorithm_params || algorithmParams.value
      if (typeof data.save_alert_image === 'boolean') {
        saveAlertImage.value = data.save_alert_image
      } else if (typeof data.save_alert_image === 'string') {
        saveAlertImage.value = data.save_alert_image === 'true'
      }
      
      // 兼容旧配置：转换旧的方向值到新的方向值
      regions.value.forEach(region => {
        if (region.type === 'line' && region.properties) {
          const oldDirection = region.properties.direction
          // 旧配置转新配置的映射
          if (oldDirection === 'left_to_right') {
            region.properties.direction = 'in'
          } else if (oldDirection === 'right_to_left') {
            region.properties.direction = 'out'
          } else if (oldDirection === 'bidirectional') {
            region.properties.direction = 'in_out'
          }
          // 如果没有direction字段，设置默认值
          if (!region.properties.direction) {
            region.properties.direction = 'in'
          }
        }
        
        // 🔧 将归一化坐标转换为像素坐标（用于画布显示）
        if (region.points && region.points.length > 0) {
          // 检查是否是归一化坐标（值在0-1之间）
          const isNormalized = region.points.every(point => 
            point[0] >= 0 && point[0] <= 1 && point[1] >= 0 && point[1] <= 1
          )
          
          if (isNormalized) {
            // 转换为像素坐标用于画布显示
            const pixelPoints = normalizedToPixel(region.points)
            console.log(`区域 ${region.name} 坐标转换:`, {
              原始归一化: region.points[0],
              转换像素: pixelPoints[0],
              画布尺寸: { canvasWidth, canvasHeight }
            })
            region.points = pixelPoints
          } else {
            console.log(`区域 ${region.name} 已是像素坐标`)
          }
        }
      })
      
      // 🔧 确保Canvas已准备好再绘制
      await nextTick()
      
      // 在画布上绘制已有区域
      regions.value.forEach(region => {
        console.log(`绘制区域: ${region.name}`, region.type, region.points)
        drawRegionOnCanvas(region)
      })
      
      message.success(`已加载 ${regions.value.length} 个配置区域`)
    } else {
      console.log('暂无已有配置')
    }
  } catch (error) {
    console.log('无已有配置或加载失败:', error)
    // 不显示错误消息，因为首次配置时是正常的
  }
}

// 绘制箭头函数 - 箭头垂直于线条，表示穿越方向
const drawArrowsForLine = (region) => {
  const [p1, p2] = region.points
  const direction = region.properties.direction || 'in'
  const color = region.properties.color || '#FF0000'
  
  // 清除该区域的旧箭头
  removeArrowsForLine(region.id)
  
  // 计算线的角度（线条本身的方向）
  const lineAngle = Math.atan2(p2[1] - p1[1], p2[0] - p1[0])
  
  // 计算线的中点
  const midX = (p1[0] + p2[0]) / 2
  const midY = (p1[1] + p2[1]) / 2
  
  // 计算垂直于线条的两个方向
  // 向上垂直方向（逆时针90度）
  const perpAngleUp = lineAngle - Math.PI / 2
  // 向下垂直方向（顺时针90度）
  const perpAngleDown = lineAngle + Math.PI / 2
  
  // 根据方向绘制箭头
  if (direction === 'in') {
    // 进入：箭头垂直线条向下（表示从上方穿过线条进入下方）
    createArrow(midX, midY, perpAngleDown, color, region.id, 'arrow')
  } else if (direction === 'out') {
    // 离开：箭头垂直线条向上（表示从下方穿过线条离开到上方）
    createArrow(midX, midY, perpAngleUp, color, region.id, 'arrow')
  } else if (direction === 'in_out') {
    // 双向：两个相反的垂直箭头
    // 向下的箭头（进入）
    const offset = 20  // 箭头间隔
    const offsetX = offset * Math.cos(lineAngle)
    const offsetY = offset * Math.sin(lineAngle)
    
    createArrow(midX - offsetX, midY - offsetY, perpAngleDown, color, region.id, 'arrow')
    createArrow(midX + offsetX, midY + offsetY, perpAngleUp, color, region.id, 'arrow')
  }
}

// 创建单个箭头
const createArrow = (x, y, angle, color, regionId, arrowType) => {
  const arrowSize = 12
  
  // 计算箭头三角形的三个点
  const points = [
    { x: x, y: y }, // 箭头尖端
    { 
      x: x - arrowSize * Math.cos(angle - Math.PI / 6),
      y: y - arrowSize * Math.sin(angle - Math.PI / 6)
    },
    { 
      x: x - arrowSize * Math.cos(angle + Math.PI / 6),
      y: y - arrowSize * Math.sin(angle + Math.PI / 6)
    }
  ]
  
  const arrow = new fabric.Polygon(points, {
    fill: color,
    stroke: color,
    strokeWidth: 1,
    selectable: false,
    evented: false,
    objectCaching: false
  })
  
  arrow.set('regionId', regionId)
  arrow.set('isArrow', true)
  canvas.add(arrow)
}

// 移除线条的箭头
const removeArrowsForLine = (regionId) => {
  const arrows = canvas.getObjects().filter(obj => 
    obj.get('regionId') === regionId && obj.get('isArrow')
  )
  arrows.forEach(arrow => canvas.remove(arrow))
}

// 更新区域箭头（当方向改变时）
const updateRegionArrow = (region) => {
  if (region.type === 'line') {
    drawArrowsForLine(region)
    canvas.renderAll()
  }
}

// 在画布上绘制区域
const drawRegionOnCanvas = (region) => {
  let shape = null
  
  if (region.type === 'line') {
    // 创建线条
    shape = new fabric.Line(
      [region.points[0][0], region.points[0][1], region.points[1][0], region.points[1][1]],
      {
        stroke: region.properties.color,
        strokeWidth: region.properties.thickness || 3,
        selectable: true,
        hasControls: true
      }
    )
    
    // 添加线条到画布
    shape.set('regionId', region.id)
    canvas.add(shape)
    
    // 绘制箭头
    drawArrowsForLine(region)
    return // 早期返回，因为已经添加到画布了
    
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
    
    // 创建区域对象
    const newRegion = {
      id: regionId,
      name: `线_${regions.value.length + 1}`,
      type: 'line',
      enabled: true,
      points: [[Math.round(lineStart.x), Math.round(lineStart.y)], [Math.round(pointer.x), Math.round(pointer.y)]],
      properties: {
        color: '#FF0000',
        thickness: 3,
        direction: 'in'  // 默认进入方向
      }
    }
    
    // 添加到区域列表
    regions.value.push(newRegion)
    
    // 绘制箭头
    drawArrowsForLine(newRegion)
    
    lineStart = null
    drawMode.value = null
    message.success('线绘制完成，可在右侧配置检测方向')
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
    if (regionId && !obj.get('isArrow')) {
      // 只处理主图形，不处理箭头（箭头会在deleteRegion中被删除）
      deleteRegion(regionId)
    }
  })
  canvas.discardActiveObject()
  canvas.renderAll()
}

// 删除区域
const deleteRegion = (regionId) => {
  const region = regions.value.find(r => r.id === regionId)
  
  // 如果是线条，先删除箭头
  if (region && region.type === 'line') {
    removeArrowsForLine(regionId)
  }
  
  regions.value = regions.value.filter(r => r.id !== regionId)
  
  // 从画布删除主图形
  const obj = canvas.getObjects().find(o => o.get('regionId') === regionId && !o.get('isArrow'))
  if (obj) {
    canvas.remove(obj)
    canvas.renderAll()
  }
  
  message.success('区域已删除')
}

// 清空全部
const clearAll = () => {
  // 清除所有区域（包括箭头）
  canvas.getObjects().forEach(obj => {
    if (obj.get('regionId') || obj.get('isArrow')) {
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
      // 更新箭头颜色
      drawArrowsForLine(region)
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
  
  if (!canvasWidth || !canvasHeight) {
    message.error('画布尺寸未初始化，请重新打开配置界面')
    return
  }
  
  saving.value = true
  try {
    // 深拷贝regions，避免修改原数据
    const regionsToSave = JSON.parse(JSON.stringify(regions.value))
    
    // 将所有区域的坐标转换为归一化坐标
    regionsToSave.forEach(region => {
      if (region.points && region.points.length > 0) {
        region.points = pixelToNormalized(region.points)
      }
    })
    
    const config = {
      task_id: props.taskInfo.id,
      task_type: props.taskInfo.task_type,
      config_version: '2.0',  // 升级到2.0表示使用归一化坐标
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      canvas_size: {
        width: canvasWidth,
        height: canvasHeight
      },
      coordinate_type: 'normalized',  // 明确标记坐标类型
      regions: regionsToSave,
      algorithm_params: algorithmParams.value,
      save_alert_image: saveAlertImage.value
    }
    
    console.log('保存配置（归一化坐标）:', {
      canvas_size: config.canvas_size,
      regions_count: regionsToSave.length,
      sample_point: regionsToSave[0]?.points[0]
    })
    
    await frameApi.saveAlgoConfig(props.taskInfo.id, config)
    message.success('配置保存成功（使用归一化坐标）')
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
  border: 2px dashed #d9d9d9;
  border-radius: 8px;
  overflow: hidden;
  background: #fafafa;
  min-height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
}

.canvas-wrapper.has-image {
  border: 2px solid #1890ff;
  background: #fff;
}

.canvas-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
}

#algo-canvas {
  display: block;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.regions-list {
  max-height: 400px;
  overflow-y: auto;
}

:deep(.ant-collapse-item) {
  margin-bottom: 8px;
}

.mb-3 {
  margin-bottom: 12px;
}
</style>

