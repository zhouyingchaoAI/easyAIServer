<template>
  <a-modal v-model:open="open" :title="isEdit ? '编辑流任务' : '创建流任务'" :confirm-loading="submitLoading"
    @cancel="handelCancel" :ok-button-props="{ htmlType: 'submit', form: 'editForm' }" centered width="800px">
    <a-form id="editForm" :model="formData" layout="vertical" @finish="onFinish" class="pt-2">
      <a-form-item name="name" label="任务名称" :rules="[{ required: true, message: '请输入任务名称' }]">
        <a-input v-model:value="formData.name" placeholder="请输入任务名称" />
      </a-form-item>

            <!-- 流名称和RTSP地址完全自动生成，无需用户填写，参考直播服务 -->

      <a-form-item name="videoPath" label="视频文件路径" :rules="[{ required: true, message: '请输入视频文件路径' }]">
        <div class="flex gap-2">
          <a-input v-model:value="formData.videoPath" placeholder="请输入视频文件绝对路径或从VOD列表选择" />
          <a-button @click="selectFromVod">从VOD选择</a-button>
          <a-button @click="selectVideoFile">浏览文件</a-button>
        </div>
        <div class="text-xs text-gray-500 mt-1">可以手动输入绝对路径，或从VOD列表选择视频文件</div>
      </a-form-item>

      <a-form-item name="loop" label="循环播放" value-prop-name="checked">
        <a-switch v-model:checked="formData.loop" />
      </a-form-item>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item name="videoCodec" label="视频编码">
            <a-select v-model:value="formData.videoCodec">
              <a-select-option value="libx264">libx264</a-select-option>
              <a-select-option value="libx265">libx265</a-select-option>
              <a-select-option value="copy">copy</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item name="audioCodec" label="音频编码">
            <a-select v-model:value="formData.audioCodec">
              <a-select-option value="aac">aac</a-select-option>
              <a-select-option value="copy">copy</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item name="preset" label="编码预设">
            <a-select v-model:value="formData.preset">
              <a-select-option value="ultrafast">ultrafast</a-select-option>
              <a-select-option value="fast">fast</a-select-option>
              <a-select-option value="medium">medium</a-select-option>
              <a-select-option value="slow">slow</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item name="tune" label="编码调优">
            <a-select v-model:value="formData.tune">
              <a-select-option value="zerolatency">zerolatency</a-select-option>
              <a-select-option value="fastdecode">fastdecode</a-select-option>
              <a-select-option value="">无</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item name="enabled" label="创建后自动启动" value-prop-name="checked">
        <a-switch v-model:checked="formData.enabled" />
      </a-form-item>

      <a-form-item name="id" hidden>
        <a-input v-model:value="formData.id" />
      </a-form-item>
    </a-form>

    <!-- 从VOD选择对话框 -->
    <a-modal v-model:open="vodSelectOpen" title="从VOD列表选择视频" @ok="confirmVodSelect" width="900px">
      <div class="mb-4">
        <a-input-search v-model:value="vodSearch" placeholder="搜索视频名称" @search="searchVod" allow-clear />
        <div class="text-xs text-gray-500 mt-2">只显示已完成转码的视频</div>
      </div>
      <a-list 
        :data-source="vodList" 
        :loading="vodLoading" 
        :pagination="{ pageSize: 10, current: vodPage, total: vodTotal, onChange: onVodPageChange, showTotal: (total) => `共 ${total} 个已完成视频` }">
        <template #renderItem="{ item }">
          <a-list-item 
            @click="selectedVod = item" 
            :class="{ 'bg-blue-50 border-blue-300': selectedVod && selectedVod.id === item.id }"
            class="cursor-pointer hover:bg-gray-50 border border-transparent rounded p-2 mb-2">
            <a-list-item-meta>
              <template #title>
                <div class="flex items-center gap-2">
                  <span class="font-medium">{{ item.name || '未命名视频' }}</span>
                  <a-tag v-if="item.status === 'done'" color="green">已完成</a-tag>
                  <a-tag v-else-if="item.status === 'transing'" color="orange">转码中</a-tag>
                  <a-tag v-else color="default">{{ item.status || '未知' }}</a-tag>
                </div>
              </template>
              <template #description>
                <div class="text-xs text-gray-500 space-y-1 mt-1">
                  <div class="truncate font-mono" :title="item.path">
                    路径: {{ item.path || item.realPath || '路径未设置' }}
                  </div>
                  <div v-if="item.duration" class="flex gap-4">
                    <span>时长: {{ formatDuration(item.duration) }}</span>
                    <span v-if="item.size">大小: {{ formatFileSize(item.size) }}</span>
                  </div>
                </div>
              </template>
            </a-list-item-meta>
            <template #actions>
              <a-button type="link" size="small" @click.stop="() => { selectedVod = item; confirmVodSelect(); }">
                选择
              </a-button>
            </template>
          </a-list-item>
        </template>
        <template #empty>
          <a-empty description="暂无已完成转码的视频，请先上传视频并等待转码完成" />
        </template>
      </a-list>
    </a-modal>

    <!-- 浏览文件对话框 -->
    <a-modal v-model:open="fileSelectOpen" title="浏览视频文件" @ok="confirmFileSelect" width="800px">
      <div class="mb-4">
        <a-input v-model:value="fileDir" placeholder="输入目录路径，如: /path/to/videos" class="mb-2" />
        <a-button type="primary" @click="loadVideoFiles">加载文件列表</a-button>
      </div>
      <a-list :data-source="videoFiles" :loading="fileLoading" :pagination="{ pageSize: 10 }">
        <template #renderItem="{ item }">
          <a-list-item @click="selectedFile = item.path" :class="{ 'bg-blue-50': selectedFile === item.path }">
            <a-list-item-meta>
              <template #title>{{ item.name }}</template>
              <template #description>{{ item.path }}</template>
            </a-list-item-meta>
          </a-list-item>
        </template>
      </a-list>
    </a-modal>
  </a-modal>
</template>

<script setup>
import { ref, reactive } from 'vue';
import { message } from 'ant-design-vue';
import { videoRtspApi, vodApi } from '@/api';
import { formatDuration, formatFileSize } from '@/utils/size';

const open = ref(false);
const isEdit = ref(false);
const fileSelectOpen = ref(false);
const vodSelectOpen = ref(false);
const selectedFile = ref('');
const selectedVod = ref(null);
const videoFiles = ref([]);
const fileLoading = ref(false);
const fileDir = ref('');
const vodList = ref([]);
const vodLoading = ref(false);
const vodSearch = ref('');
const vodPage = ref(1);
const vodTotal = ref(0);

const emit = defineEmits(['refresh']);

const formData = reactive({
  id: '',
  name: '',
  streamName: '',
  videoPath: '',
  loop: true,
  videoCodec: 'libx264',
  audioCodec: 'aac',
  preset: 'ultrafast',
  tune: 'zerolatency',
  enabled: false,
})

// 打开对话框
function openDialog(data = null) {
  if (data) {
    isEdit.value = true;
    formData.id = data.id;
    formData.name = data.name;
    formData.streamName = data.streamName;
    formData.videoPath = data.videoPath;
    formData.loop = data.loop;
    formData.videoCodec = data.videoCodec || 'libx264';
    formData.audioCodec = data.audioCodec || 'aac';
    formData.preset = data.preset || 'ultrafast';
    formData.tune = data.tune || 'zerolatency';
    formData.enabled = data.enabled || false;
  } else {
    isEdit.value = false;
    formData.id = '';
    formData.name = '';
    formData.streamName = '';
    formData.videoPath = '';
    formData.loop = true;
    formData.videoCodec = 'libx264';
    formData.audioCodec = 'aac';
    formData.preset = 'ultrafast';
    formData.tune = 'zerolatency';
    formData.enabled = false;
  }
  open.value = true;
}

// 从VOD列表选择
const selectFromVod = () => {
  vodSelectOpen.value = true;
  selectedVod.value = null;
  loadVodList();
};

// 加载VOD列表
const loadVodList = async () => {
  vodLoading.value = true;
  try {
    const params = {
      start: (vodPage.value - 1) * 10,
      limit: 50, // 获取更多数据以便过滤
      q: vodSearch.value,
    };
    const res = await vodApi.getVodList(params);
    
    console.log('VOD API响应:', res);
    
    // VOD API返回的数据结构: { total: number, rows: array }
    // axios会自动将响应包装在 response.data 中
    let rows = [];
    let total = 0;
    
    if (res && res.data) {
      // 检查响应数据结构
      if (res.data.rows && Array.isArray(res.data.rows)) {
        rows = res.data.rows;
        total = res.data.total || 0;
      } else if (Array.isArray(res.data)) {
        // 如果是数组直接返回
        rows = res.data;
        total = res.data.length;
      }
    }
    
    console.log('解析后的VOD数据:', { rows: rows.length, total, sample: rows[0] });
    
    // 只显示已完成的视频（status === 'done'）
    const doneVideos = rows.filter(item => {
      if (!item) return false;
      // 检查status字段，可能的值: 'done', 'transing', 'waiting'等
      return item.status === 'done';
    });
    
    vodList.value = doneVideos;
    // 使用已完成视频的数量作为总数（简单处理，实际应该后端过滤）
    vodTotal.value = doneVideos.length;
    
    if (doneVideos.length === 0) {
      if (rows.length > 0) {
        const statuses = [...new Set(rows.map(item => item?.status).filter(Boolean))];
        message.warning(`当前页面没有已完成转码的视频（状态: ${statuses.join(', ')}），请等待转码完成或翻页查看`);
      } else {
        message.info('暂无视频文件，请先上传视频');
      }
    }
  } catch (err) {
    console.error('加载VOD列表失败', err);
    console.error('错误详情:', {
      response: err.response,
      data: err.response?.data,
      status: err.response?.status,
    });
    const errorMsg = err.response?.data?.msg || err.message || '未知错误';
    message.error('加载VOD列表失败: ' + errorMsg);
  } finally {
    vodLoading.value = false;
  }
};

// VOD搜索
const searchVod = () => {
  vodPage.value = 1;
  loadVodList();
};

// VOD翻页
const onVodPageChange = (page) => {
  vodPage.value = page;
  loadVodList();
};

// 确认选择VOD
const confirmVodSelect = () => {
  if (!selectedVod.value) {
    message.warning('请选择一个视频');
    return;
  }
  
  console.log('选中的VOD数据:', selectedVod.value);
  
  // VOD返回的path字段就是视频文件的绝对路径（RealPath映射到path）
  let videoPath = selectedVod.value.path;
  
  // 如果path为空，尝试从其他字段获取
  if (!videoPath || videoPath.trim() === '') {
    // 尝试从RealPath获取（如果存在）
    videoPath = selectedVod.value.realPath || selectedVod.value.RealPath;
    
    // 如果还是没有，尝试构建路径
    if ((!videoPath || videoPath.trim() === '') && selectedVod.value.folder && selectedVod.value.id) {
      // 这种情况不应该发生，但作为后备方案
      console.warn('VOD path字段为空，无法自动构建路径');
      message.error('该视频文件路径为空，请手动输入路径');
      return;
    }
  }
  
  if (!videoPath || videoPath.trim() === '') {
    message.error('该视频文件路径为空，无法使用');
    console.error('选中的VOD数据:', selectedVod.value);
    return;
  }
  
  formData.videoPath = videoPath;
  
  // 如果任务名称为空，使用视频名称
  if (!formData.name || formData.name.trim() === '') {
    formData.name = (selectedVod.value.name || '未命名视频') + ' - RTSP流';
  }
  
  vodSelectOpen.value = false;
  message.success('已选择视频: ' + (selectedVod.value.name || '未知'));
};

// 浏览文件
const selectVideoFile = () => {
  fileSelectOpen.value = true;
  selectedFile.value = formData.videoPath;
};

// 加载视频文件列表
const loadVideoFiles = async () => {
  if (!fileDir.value) {
    message.warning('请输入目录路径');
    return;
  }
  fileLoading.value = true;
  try {
    const res = await videoRtspApi.getVideoFiles(fileDir.value);
    if (res.data.code === 200) {
      videoFiles.value = res.data.data || [];
      if (videoFiles.value.length === 0) {
        message.info('该目录下没有找到视频文件');
      }
    }
  } catch (err) {
    console.error('加载视频文件失败', err);
    message.error(err.response?.data?.msg || '加载视频文件失败');
  } finally {
    fileLoading.value = false;
  }
};

// 确认选择文件
const confirmFileSelect = () => {
  if (selectedFile.value) {
    formData.videoPath = selectedFile.value;
    fileSelectOpen.value = false;
  } else {
    message.warning('请选择一个视频文件');
  }
};

// 提交
const submitLoading = ref(false);
async function onFinish(values) {
  if (submitLoading.value) return;
  submitLoading.value = true;

          // 验证必填字段（使用formData，因为values可能不完整）
          if (!formData.name || !formData.name.trim()) {
            message.error('请输入任务名称');
            submitLoading.value = false;
            return;
          }
          if (!formData.videoPath || !formData.videoPath.trim()) {
            message.error('请选择视频文件');
            submitLoading.value = false;
            return;
          }

          // 准备提交数据（合并formData和values，确保所有字段都包含）
          // 流名称完全由后端自动生成，前端不传递（与直播服务一致）
          const submitData = {
            name: formData.name.trim(),
            streamName: '', // 留空，后端自动生成（格式：video_<uuid前12位>，避免与直播服务的 stream_<id> 冲突）
            videoPath: formData.videoPath.trim(),
    loop: formData.loop !== undefined ? formData.loop : true,
    videoCodec: formData.videoCodec || 'libx264',
    audioCodec: formData.audioCodec || 'aac',
    preset: formData.preset || 'ultrafast',
    tune: formData.tune || 'zerolatency',
    enabled: formData.enabled !== undefined ? formData.enabled : false,
  };

  console.log('提交的数据:', submitData);
  console.log('视频路径:', submitData.videoPath);

  try {
    if (isEdit.value) {
      await videoRtspApi.updateStream(formData.id, submitData);
      message.success('更新成功');
    } else {
      const result = await videoRtspApi.createStream(submitData);
      console.log('创建结果:', result);
      message.success('创建成功');
    }

    emit('refresh');
    handelCancel();
  } catch (err) {
    console.error('提交失败', err);
    console.error('错误详情:', err.response);
    console.error('请求数据:', submitData);
    
    // 提取更详细的错误信息
    let errorMsg = '操作失败';
    if (err.response?.data) {
      // 尝试从不同位置获取错误消息
      if (err.response.data.msg) {
        errorMsg = err.response.data.msg;
      } else if (err.response.data.message) {
        errorMsg = err.response.data.message;
      } else if (err.response.data.error) {
        errorMsg = err.response.data.error;
      } else if (typeof err.response.data === 'string') {
        errorMsg = err.response.data;
      }
      // 尝试获取details中的错误信息
      if (err.response.data.details && Array.isArray(err.response.data.details) && err.response.data.details.length > 0) {
        errorMsg = err.response.data.details.join('; ');
      }
    } else if (err.message) {
      errorMsg = err.message;
    }
    
    message.error(errorMsg);
  } finally {
    submitLoading.value = false;
  }
}

function handelCancel() {
  open.value = false;
  fileSelectOpen.value = false;
  vodSelectOpen.value = false;
  selectedFile.value = '';
  selectedVod.value = null;
  videoFiles.value = [];
  vodList.value = [];
  fileDir.value = '';
  vodSearch.value = '';
  vodPage.value = 1;
}

defineExpose({
  open: openDialog,
});
</script>

