<template>
  <div>
    <div class="bg-white rounded-md cursor-pointer p-2 flex justify-between items-center">
      <div class="flex gap-2">
        <a-button type="primary" @click="onClickCreate">
          <template #icon>
            <PlusOutlined />
          </template>
          创建流任务
        </a-button>
        <a-button @click="onClickUpload">
          <template #icon>
            <UploadOutlined />
          </template>
          上传视频
        </a-button>
      </div>

      <a-input-search class="w-68" v-model:value="streamParams.q" placeholder="请输入任务名称" enter-button @search="onSearch" />
    </div>

    <div class="mt-5">
      <template v-if="streamData.items.length > 0">
        <a-row :gutter="[16, 16]">
          <a-col :xs="24" :sm="24" :md="12" :lg="8" :xl="6" :xxl="4" v-for="(item, index) in streamData.items"
            :key="item.id">
            <StreamCard :data="item" @on-start="onStartStream" @on-stop="onStopStream" @on-delete="onDeleteStream"
              @on-edit="onEdit" @refresh="getStreamDataList" />
          </a-col>
        </a-row>
      </template>
      <template v-else>
        <div class="p-2 bg-white rounded-md">
          <a-empty :image="simpleImage" />
        </div>
      </template>
    </div>
    <a-pagination class="mt-4 text-right" :current="currentPage" :page-size="streamParams.limit" :total="streamData.total"
      show-less-items :show-total="total => `共 ${total} 项`" @change="onPageChange" />

    <StreamEdit ref="editRef" @refresh="getStreamDataList" />
    <VodUploadModal :open="uploadModalVisible" @refreshList="onVodUploadSuccess" @update:open="uploadModalVisible = false" />
  </div>
</template>

<script setup>
import { onMounted, reactive, ref, watch } from 'vue';
import { videoRtspApi } from '@/api';
import { PlusOutlined, UploadOutlined } from '@ant-design/icons-vue';
import StreamCard from './card.vue';
import StreamEdit from './edit.vue';
import VodUploadModal from '@/views/vod/upload.vue';
import { message } from 'ant-design-vue';
import { debounce } from 'lodash-es'
import { Empty } from 'ant-design-vue';
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

const editRef = ref();
const uploadModalVisible = ref(false);

//获取流任务数据请求参数
const currentPage = ref(1);
const streamParams = reactive({
  start: 0,
  limit: 12,
  sort: "",
  order: "",
  q: "",
});

const streamData = reactive({
  items: [],
  total: 0,
});

onMounted(() => {
  getStreamDataList();
})

// 拉取列表
function getStreamDataList() {
  streamParams.start = (currentPage.value - 1) * streamParams.limit;

  videoRtspApi.getStreamList(streamParams)
    .then(res => {
      if (res.data.code === 200) {
        streamData.items = res.data.data.rows || [];
        streamData.total = res.data.data.total || 0;
      }
    })
    .catch(err => {
      console.error(err);
      message.error('获取列表失败');
    });
}

// 翻页
const onPageChange = (page) => {
  currentPage.value = page;
  getStreamDataList();
};

// 搜索
const onSearch = (e) => {
  getStreamDataList();
}

// 防抖包装
const debounceSearch = debounce(() => {
  currentPage.value = 1;
  getStreamDataList();
}, 500);

// 监听搜索词变化
watch(() => streamParams.q, () => {
  debounceSearch();
});

const onClickCreate = () => {
  editRef.value.open();
}

const onClickUpload = () => {
  uploadModalVisible.value = true;
}

const onVodUploadSuccess = () => {
  message.success('视频上传成功，现在可以从VOD列表选择该视频创建RTSP流');
}

// 点击编辑
const onEdit = (item) => {
  editRef.value.open(item);
}

// 点击删除
const onDeleteStream = (id) => {
  videoRtspApi.deleteStream(id).then(res => {
    if (res.data.code == 200) {
      message.success('删除成功');
      getStreamDataList()
    } else {
      message.error(res.data.msg || '删除失败');
    }
  }).catch(err => {
    message.error('删除失败')
  })
}

// 启动流
const onStartStream = (id) => {
  videoRtspApi.startStream(id).then(res => {
    if (res.data.code == 200) {
      message.success('启动成功');
      getStreamDataList()
    } else {
      message.error(res.data.msg || '启动失败');
    }
  }).catch(err => {
    message.error('启动失败')
  })
}

// 停止流
const onStopStream = (id) => {
  videoRtspApi.stopStream(id).then(res => {
    if (res.data.code == 200) {
      message.success('停止成功');
      getStreamDataList()
    } else {
      message.error(res.data.msg || '停止失败');
    }
  }).catch(err => {
    message.error('停止失败')
  })
}
</script>

